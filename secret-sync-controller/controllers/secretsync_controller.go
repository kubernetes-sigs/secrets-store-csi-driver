/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"golang.org/x/crypto/pbkdf2"
	corev1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	secretsstorecsiv1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
	"sigs.k8s.io/secrets-store-csi-driver/provider/v1alpha1"
	secretsyncv1alpha1 "sigs.k8s.io/secrets-store-csi-driver/secret-sync-controller/api/v1alpha1"
	"sigs.k8s.io/secrets-store-csi-driver/secret-sync-controller/pkg/k8s"
	"sigs.k8s.io/secrets-store-csi-driver/secret-sync-controller/pkg/provider"
	"sigs.k8s.io/secrets-store-csi-driver/secret-sync-controller/pkg/util/secretutil"
	"sigs.k8s.io/secrets-store-csi-driver/secret-sync-controller/pkg/util/tokenutil"
)

const (
	// FilePermission is the permission to be used for the staging target path
	FilePermission os.FileMode = 0644

	// CSIPodName is the name of the pod that the mount is created for
	CSIPodName = "csi.storage.k8s.io/pod.name"

	// CSIPodNamespace is the namespace of the pod that the mount is created for
	CSIPodNamespace = "csi.storage.k8s.io/pod.namespace"

	// CSIPodUID is the UID of the pod that the mount is created for
	CSIPodUID = "csi.storage.k8s.io/pod.uid"

	// CSIPodServiceAccountName is the name of the pod service account that the mount is created for
	CSIPodServiceAccountName = "csi.storage.k8s.io/serviceAccount.name"

	// CSIPodServiceAccountTokens is the service account tokens of the pod that the mount is created for
	CSIPodServiceAccountTokens = "csi.storage.k8s.io/serviceAccount.tokens" //nolint

	// Label applied by the controller to the secret object
	ControllerLabelKey = "secrets-store.sync.x-k8s.io"

	// Value of the label applied by the controller to the secret object
	ControllerLabelValue = ""

	// Label applied by the controller to the secret object
	ControllerAnnotationKey = "secrets-store.sync.x-k8s.io"

	// Value of the label applied by the controller to the secret object
	ControllerAnnotationValue = ""

	// Version is the version of the secret sync controller
	Version = "v1"

	// SecretSyncControllerFieldManager is the field manager used by the secret sync controller
	SecretSyncControllerFieldManager = Version + "-secret-sync-controller"
)

type AllClientBuilder interface {
	Get(ctx context.Context, provider string) (v1alpha1.CSIDriverProviderClient, error)
}

// SecretSyncReconciler reconciles a SecretSync object
type SecretSyncReconciler struct {
	client.Client
	Scheme               *runtime.Scheme
	TokenClient          *k8s.TokenClient
	ProviderClients      AllClientBuilder
	TokenRequest         *tokenutil.TokenRequest
	RotationPollInterval time.Duration
	EventRecorder        record.EventRecorder
}

//+kubebuilder:rbac:groups=secret-sync.x-k8s.io,resources=secretsyncs,verbs=get;list;watch
//+kubebuilder:rbac:groups=secret-sync.x-k8s.io,resources=secretsyncs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=create;patch;delete
//+kubebuilder:rbac:groups="",resources="serviceaccounts/token",verbs=create
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=secrets-store.csi.x-k8s.io,resources=secretproviderclasses,verbs=get;list;watch

func (r *SecretSyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	logger := log.FromContext(ctx)

	logger.Info("Reconciling SecretSync")

	// get the secret sync object
	ss := &secretsyncv1alpha1.SecretSync{}
	if err := r.Get(ctx, req.NamespacedName, ss); err != nil {
		logger.Error(err, "unable to fetch SecretSync")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// update status conditions
	r.updateStatusConditions(ctx, ss, "", ConditionTypeUnknown, ConditionReasonUnknown, false)

	// if the secret sync hash is empty, it means the secret does not exist, so the condition type is create
	// otherwise, the condition type is update
	conditionType := ConditionTypeUpdate
	if ss.Status.SyncHash == "" {
		conditionType = ConditionTypeCreate
	}

	// get the secret provider class object
	spc := &secretsstorecsiv1.SecretProviderClass{}
	if err := r.Get(ctx, client.ObjectKey{Name: ss.Spec.SecretProviderClassName, Namespace: req.Namespace}, spc); err != nil {
		logger.Error(err, "failed to get secret provider class", "name", ss.Spec.SecretProviderClassName)
		r.updateStatusConditions(ctx, ss, ConditionTypeUnknown, conditionType, ConditionReasonControllerSpcError, true)
		return ctrl.Result{}, err
	}

	secretName := strings.TrimSpace(ss.Name)

	// get the service account token
	serviceAccountTokenAttrs, err := r.TokenClient.SecretProviderServiceAccountTokenAttrs(ss.Namespace, ss.Spec.ServiceAccountName, r.TokenRequest)
	if err != nil {
		logger.Error(err, "failed to get service account token", "name", ss.Spec.ServiceAccountName)

		if checkIfErrorMessageCanBeDisplayed(err.Error()) {
			r.updateStatusConditions(ctx, ss, ConditionTypeUnknown, conditionType, ConditionReasonValidatingAdmissionPolicyCheckFailed, true)
		} else {
			r.updateStatusConditions(ctx, ss, ConditionTypeUnknown, conditionType, ConditionReasonSecretPatchFailedUnknownError, true)
		}
		return ctrl.Result{}, err
	}

	// this is to mimic the parameters sent from CSI driver to the provider
	parameters := make(map[string]string)
	if spc.Spec.Parameters != nil {
		parameters = spc.Spec.Parameters
	}

	parameters[CSIPodName] = os.Getenv("SYNC_CONTROLLER_POD_NAME")
	parameters[CSIPodUID] = os.Getenv("SYNC_CONTROLLER_POD_UID")
	parameters[CSIPodNamespace] = req.Namespace
	parameters[CSIPodServiceAccountName] = ss.Spec.ServiceAccountName

	for k, v := range serviceAccountTokenAttrs {
		parameters[k] = v
	}

	paramsJSON, err := json.Marshal(parameters)
	if err != nil {
		logger.Error(err, "failed to marshal parameters", "parameters", parameters)
		r.updateStatusConditions(ctx, ss, ConditionTypeUnknown, conditionType, ConditionReasonControllerInternalError, true)
		return ctrl.Result{}, err
	}

	permissionJSON, err := json.Marshal(FilePermission)
	if err != nil {
		logger.Error(err, "failed to marshal file permission", "filePermission", FilePermission)
		r.updateStatusConditions(ctx, ss, ConditionTypeUnknown, conditionType, ConditionReasonControllerInternalError, true)
		return ctrl.Result{}, err
	}

	providerName := string(spc.Spec.Provider)
	providerClient, err := r.ProviderClients.Get(ctx, providerName)
	if err != nil {
		logger.Error(err, "failed to get provider client", "provider", providerName)
		r.updateStatusConditions(ctx, ss, ConditionTypeUnknown, conditionType, ConditionReasonControllerSpcError, true)
		return ctrl.Result{}, err
	}

	secretRefData := make(map[string]string)
	var secretsJSON []byte
	secretsJSON, err = json.Marshal(secretRefData)
	if err != nil {
		logger.Error(err, "failed to marshal secret")
		r.updateStatusConditions(ctx, ss, ConditionTypeUnknown, conditionType, ConditionReasonControllerInternalError, true)
		return ctrl.Result{}, err
	}

	oldObjectVersions := make(map[string]string)

	objectVersions, files, err := provider.MountContent(ctx, providerClient, string(paramsJSON), string(secretsJSON), string(permissionJSON), oldObjectVersions)
	if err != nil {
		logger.Error(err, "failed to get secrets from provider", "provider", providerName)
		r.updateStatusConditions(ctx, ss, ConditionTypeUnknown, conditionType, ConditionReasonFailedProviderError, true)
		return ctrl.Result{}, err
	}

	secretObj := ss.Spec.SecretObject
	if err = secretutil.ValidateSecretObject(secretName, secretObj); err != nil {
		logger.Error(err, "failed to validate secret object", "secretName", secretName)
		r.updateStatusConditions(ctx, ss, ConditionTypeUnknown, conditionType, ConditionReasonUserInputValidationFailed, true)
		return ctrl.Result{}, err
	}

	secretType := secretutil.GetSecretType(strings.TrimSpace(secretObj.Type))
	var datamap map[string][]byte
	if datamap, err = secretutil.GetSecretData(secretObj.Data, secretType, files); err != nil {
		logger.Error(err, "failed to get secret data", "secretName", secretName)
		r.updateStatusConditions(ctx, ss, ConditionTypeUnknown, conditionType, ConditionReasonUserInputValidationFailed, true)
		return ctrl.Result{}, err
	}

	labels := make(map[string]string)
	if secretObj.Labels != nil {
		labels = secretObj.Labels
	}
	if val, ok := labels[ControllerLabelKey]; ok {
		// if the label is already present, check if it is the same as the controller label value
		// if it is not the same, return an error
		if val != ControllerLabelValue {
			r.updateStatusConditions(ctx, ss, ConditionTypeUnknown, conditionType, ConditionReasonFailedInvalidLabelError, true)
			return ctrl.Result{}, fmt.Errorf("label %s is reserved for use by the secret sync controller", ControllerLabelKey)
		}
	} else {
		// if the label is not present, add it
		labels[ControllerLabelKey] = ControllerLabelValue
	}

	annotations := make(map[string]string)
	if secretObj.Annotations != nil {
		annotations = secretObj.Annotations
	}
	if val, ok := annotations[ControllerAnnotationKey]; ok {
		// if the annotation is already present, check if it is the same as the controller annotation value
		// if it is not the same, return an error
		if val != ControllerLabelValue {
			r.updateStatusConditions(ctx, ss, ConditionTypeUnknown, conditionType, ConditionReasonFailedInvalidAnnotationError, true)
			return ctrl.Result{}, fmt.Errorf("annotation %s is reserved for use by the secret sync controller", ControllerAnnotationValue)
		}
	} else {
		// if the annotation is not present, add it
		annotations[ControllerAnnotationKey] = ControllerAnnotationValue
	}

	// Compute the hash of the secret
	syncHash, err := r.computeSecretDataObjectHash(datamap, spc, ss)
	if err != nil {
		logger.Error(err, "failed to compute secret data object hash", "secretName", secretName)
		return ctrl.Result{}, err
	}

	// Check if the hash has changed.
	hashChanged := false
	if syncHash != ss.Status.SyncHash {
		hashChanged = true
	}

	// Check if a secret create or update failed and if the controller should re-try the operation
	failedCondition := metav1.Condition{}
	for _, ssCondition := range ss.Status.Conditions {
		if slices.Contains(FailedConditionsTriggeringRetry, ssCondition.Reason) {
			failedCondition = ssCondition
			break
		}
	}

	if failedCondition.Type != "" || hashChanged {
		if conditionType == ConditionTypeCreate {
			r.updateStatusConditions(ctx, ss, ConditionTypeUnknown, conditionType, ConditionReasonCreateSucceeded, false)
		} else if hashChanged {
			r.updateStatusConditions(ctx, ss, ConditionTypeUnknown, conditionType, ConditionReasonUpdateValueChangeOrForceUpdateSucceeded, false)
		}

		// Save current state for potential rollback.
		prevSecretHash := ss.Status.SyncHash
		prevTime := ss.Status.LastSuccessfulSyncTime

		// Update status fields.
		ss.Status.LastSuccessfulSyncTime = &metav1.Time{Time: time.Now()}
		ss.Status.SyncHash = syncHash

		if failedCondition.Type != "" {
			meta.RemoveStatusCondition(&ss.Status.Conditions, failedCondition.Type)
		}

		// Attempt to create or update the secret.
		if err = r.serverSidePatchSecret(ctx, ss, secretName, req.Namespace, datamap, objectVersions, labels, annotations, secretType); err != nil {
			logger.Error(err, "failed to patch secret", "secretName", secretName)

			// Rollback to the previous hash and the previous last successful sync time.
			ss.Status.SyncHash = prevSecretHash
			ss.Status.LastSuccessfulSyncTime = prevTime

			// Reset the create or update conditions
			meta.RemoveStatusCondition(&ss.Status.Conditions, ConditionTypeCreate)
			meta.RemoveStatusCondition(&ss.Status.Conditions, ConditionTypeUpdate)

			if checkIfErrorMessageCanBeDisplayed(err.Error()) {
				failedCondition.Message = err.Error()
				r.updateStatusConditions(ctx, ss, ConditionTypeUnknown, conditionType, ConditionReasonValidatingAdmissionPolicyCheckFailed, true)
			} else {
				r.updateStatusConditions(ctx, ss, ConditionTypeUnknown, conditionType, ConditionReasonSecretPatchFailedUnknownError, true)
			}

			return ctrl.Result{}, err
		}

		// No errors found, remove the failed conditions.
		for _, cond := range ss.Status.Conditions {
			if slices.Contains(FailedConditionsTriggeringRetry, cond.Reason) {
				meta.RemoveStatusCondition(&ss.Status.Conditions, cond.Type)
			}
		}
	}

	// Update the status.
	err = r.Client.Status().Update(ctx, ss)
	if err != nil {
		return ctrl.Result{}, err
	}

	logger.V(4).Info("Done... updated status", "syncHash", syncHash, "lastSuccessfulSyncTime", ss.Status.LastSuccessfulSyncTime)
	return ctrl.Result{}, nil
}

// checkIfErrorMessageCanBeDisplayed checks if the error message can be displayed in the condition message
// based on the allowed strings to display condition error message defined in the conditions.go file.
func checkIfErrorMessageCanBeDisplayed(errorMessage string) bool {
	for _, allowedString := range AllowedStringsToDisplayConditionErrorMessage {
		if strings.Contains(strings.ToLower(errorMessage), allowedString) {
			return true
		}
	}
	return false
}

// serverSidePatchSecret performs a server-side patch on a Kubernetes Secret.
// It updates the specified secret with the provided data, labels, and annotations.
func (r *SecretSyncReconciler) serverSidePatchSecret(ctx context.Context, ss *secretsyncv1alpha1.SecretSync, name, namespace string, datamap map[string][]byte, _, labels, annotations map[string]string, secretType corev1.SecretType) (err error) {
	logger := log.FromContext(ctx)

	secretKind := "Secret"
	secretVersion := "v1"

	// Retrieve Kubernetes clientset.
	clientset := kubernetes.NewForConfigOrDie(ctrl.GetConfigOrDie())
	coreV1Client := clientset.CoreV1()

	// Construct the patch for updating the Secret.
	secretPatchData := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       secretKind,
			APIVersion: secretVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: ss.APIVersion,
					Kind:       ss.Kind,
					Name:       ss.Name,
					UID:        ss.UID,
				},
			},
		},
		Data: datamap,
		Type: secretType,
	}

	patchData, err := json.Marshal(secretPatchData)
	if err != nil {
		logger.Error(err, "failed to marshal secretPatchData")
		return err
	}

	// Perform the server-side patch on the Secret.
	_, err = coreV1Client.Secrets(namespace).Patch(ctx, name, types.ApplyPatchType, patchData, metav1.PatchOptions{FieldManager: SecretSyncControllerFieldManager})
	if err != nil {
		logger.Error(err, "failed to patch secret")
		return err
	}

	return nil
}

// computeSecretDataObjectHash computes the HMAC hash of the provided secret data
// using the SS UID as the key.
func (r *SecretSyncReconciler) computeSecretDataObjectHash(secretData map[string][]byte, spc *secretsstorecsiv1.SecretProviderClass, ss *secretsyncv1alpha1.SecretSync) (string, error) {
	// Serialize the secret data, parts of the spc and the ss data.
	secretBytes, err := json.Marshal(secretData)
	if err != nil {
		return "", err
	}

	spcBytesUID, err := json.Marshal(spc.UID)
	if err != nil {
		return "", err
	}
	secretBytes = append(secretBytes, spcBytesUID...)

	spcBytesGeneration, err := json.Marshal(spc.ObjectMeta.Generation)
	if err != nil {
		return "", err
	}
	secretBytes = append(secretBytes, spcBytesGeneration...)

	ssBytesUID, err := json.Marshal(ss.UID)
	if err != nil {
		return "", err
	}
	secretBytes = append(secretBytes, ssBytesUID...)

	ssBytesGeneration, err := json.Marshal(ss.ObjectMeta.Generation)
	if err != nil {
		return "", err
	}
	secretBytes = append(secretBytes, ssBytesGeneration...)

	ssBytesForceSync, err := json.Marshal(ss.Spec.ForceSynchronization)
	if err != nil {
		return "", err
	}
	secretBytes = append(secretBytes, ssBytesForceSync...)

	salt := []byte(string(ss.UID))
	dk := pbkdf2.Key(secretBytes, salt, 100_000, 32, sha512.New)

	// Create a new HMAC instance with SHA-56 as the hash type and the pbkdf2 key.
	hmac := hmac.New(sha512.New, dk)

	_, err = hmac.Write(dk)
	if err != nil {
		return "", err
	}

	// Get the final HMAC hash in hexadecimal format.
	dataHmac := hmac.Sum(nil)
	dataHmac = append([]byte(Version), dataHmac...)
	hmacHex := hex.EncodeToString(dataHmac)

	return hmacHex, nil
}

// processIfSecretChanged checks if the secret sync object has changed.
func (r *SecretSyncReconciler) processIfSecretChanged(oldObj, newObj client.Object) bool {
	ssOldObj := oldObj.(*secretsyncv1alpha1.SecretSync)
	ssNewObj := newObj.(*secretsyncv1alpha1.SecretSync)

	return ssNewObj.Status.SyncHash != ssOldObj.Status.SyncHash
}

// We need to trigger the reconcile function when the secret sync object is created or updated, however
// we don't need to trigger the reconcile function when the status of the secret sync object is updated.
func (r *SecretSyncReconciler) shouldReconcilePredicate() predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return r.processIfSecretChanged(e.ObjectOld, e.ObjectNew)
		},
		DeleteFunc: func(_ event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(_ event.GenericEvent) bool {
			return true
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *SecretSyncReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&secretsyncv1alpha1.SecretSync{}).
		WithEventFilter(r.shouldReconcilePredicate()).
		Complete(r)
}
