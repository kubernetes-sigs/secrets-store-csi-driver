// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package controllers

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/secrets-store-csi-driver/apis/v1alpha1"
)

const (
	certType       = "CERTIFICATE"
	privateKeyType = "RSA PRIVATE KEY"
)

// SecretProviderClassPodStatusReconciler reconciles a SecretProviderClassPodStatus object
type SecretProviderClassPodStatusReconciler struct {
	client.Client
	Log    log.Logger
	Scheme *runtime.Scheme
	NodeID string
}

// +kubebuilder:rbac:groups=secrets-store.csi.x-k8s.io,resources=secretproviderclasspodstatuses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=secrets-store.csi.x-k8s.io,resources=secretproviderclasspodstatuses/status,verbs=get;update;patch

func (r *SecretProviderClassPodStatusReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	logger := log.WithFields(log.Fields{"secretproviderclasspodstatus": req.NamespacedName, "node": r.NodeID})
	logger.Info("reconcile started")

	var spcPodStatus v1alpha1.SecretProviderClassPodStatus
	// get the secret provider class pod status object
	if err := r.Get(ctx, req.NamespacedName, &spcPodStatus); err != nil {
		// object not found
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	node, ok := spcPodStatus.GetLabels()["internal.secrets-store.csi.k8s.io/node-name"]
	if !ok {
		logger.Info("node label not found, ignoring this spc pod status")
		return ctrl.Result{}, nil
	}
	if !strings.EqualFold(node, r.NodeID) {
		logger.Infof("ignoring as spc pod status belongs to node %s", node)
		return ctrl.Result{}, nil
	}

	// reconcile delete
	// TODO (aramase)  what to do on delete of the spc pod status object
	// What action to take on the spc for this
	if !spcPodStatus.GetDeletionTimestamp().IsZero() {
		logger.Info("reconcile delete complete")
		return ctrl.Result{}, nil
	}

	// targetPath := spcPodStatus.Status.TargetPath
	spcName := spcPodStatus.Status.SecretProviderClassName
	spc := &v1alpha1.SecretProviderClass{}
	// get the secret provider class object
	if err := r.Client.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: spcName}, spc); err != nil {
		logger.Errorf("failed to get spc, error: %+v", err)
		// object not found
		if apierrors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
		return ctrl.Result{}, err
	}

	if len(spc.Spec.SecretObjects) == 0 {
		logger.Infof("No secret objects defined for spc, nothing to reconcile")
		return ctrl.Result{}, nil
	}
	secretStatus := make(map[string]bool, 0)
	for _, secretRef := range spc.Status.SecretRef {
		secretStatus[secretRef.Name] = true
	}

	files, err := getMountedFiles(spcPodStatus.Status.TargetPath)
	if err != nil {
		logger.Errorf("failed to get mounted files, err: %+v", err)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}
	errs := make([]error, 0)
	success := make([]string, 0)
	for idx, secretObj := range spc.Spec.SecretObjects {
		_, ok := secretStatus[secretObj.SecretName]
		// secret has already been created
		if ok {
			err = r.patchSecretWithOwnerRef(ctx, secretObj.SecretName, req.Namespace, &spcPodStatus, r.Scheme)
			if err != nil {
				logger.Errorf("failed to set owner ref for secret, err: %+v", err)
				return ctrl.Result{RequeueAfter: 10 * time.Second}, err
			}
			continue
		}
		if len(secretObj.SecretName) == 0 {
			logger.Errorf("secret name is empty at index %d", idx)
			errs = append(errs, fmt.Errorf("secret name is empty at index %d", idx))
			continue
		}
		if len(secretObj.Type) == 0 {
			logger.Errorf("secret type is empty at index %d for secret %s", idx, secretObj.SecretName)
			errs = append(errs, fmt.Errorf("secret type is empty at index %d for secret %s", idx, secretObj.SecretName))
			continue
		}
		if len(secretObj.Data) == 0 {
			logger.Errorf("data is empty at index %d for secret %s", idx, secretObj.SecretName)
			errs = append(errs, fmt.Errorf("data is empty at index %d for secret %s", idx, secretObj.SecretName))
			continue
		}
		secretType := getSecretType(secretObj.Type)
		datamap := make(map[string][]byte)

		logger.Infof("secret type is %s for secret %s", secretType, secretObj.SecretName)

		for _, data := range secretObj.Data {
			if len(data.ObjectName) == 0 {
				logger.Errorf("object name in data is empty at index %d for secret %s", idx, secretObj.SecretName)
				errs = append(errs, fmt.Errorf("object name in data is empty at index %d for secret %s", idx, secretObj.SecretName))
				continue
			}
			if len(data.Key) == 0 {
				logger.Errorf("key in data is empty at index %d for secret %s", idx, secretObj.SecretName)
				errs = append(errs, fmt.Errorf("key in data is empty at index %d for secret %s", idx, secretObj.SecretName))
				continue
			}
			file, ok := files[data.ObjectName]
			if !ok {
				logger.Errorf("file matching objectName %s not found for secret %s", data.ObjectName, secretObj.SecretName)
				continue
			}
			logger.Infof("file matching objectName %s found for key %s", data.ObjectName, data.Key)
			content, err := ioutil.ReadFile(file)
			if err != nil {
				logger.Errorf("failed to read file %s, err: %v", data.ObjectName, err)
				return ctrl.Result{}, status.Error(codes.Internal, err.Error())
			}
			datamap[data.Key] = content
			if secretType == corev1.SecretTypeTLS {
				c, err := getCertPart(content, data.Key)
				if err != nil {
					logger.Errorf("failed to get cert data from file %s, err: %v for secret: %s", file, err, secretObj.SecretName)
					return ctrl.Result{RequeueAfter: 5 * time.Second}, status.Error(codes.Internal, err.Error())
				}
				datamap[data.Key] = c
			}
		}

		createFn := func() (bool, error) {
			if err := r.createOrUpdateK8sSecret(ctx, secretObj.SecretName, req.Namespace, datamap, secretType); err != nil {
				log.Errorf("failed createOrUpdateK8sSecret, err: %v for secret: %s", err, secretObj.SecretName)
				return false, nil
			}
			return true, nil
		}
		if err := wait.ExponentialBackoff(wait.Backoff{
			Steps:    5,
			Duration: 1 * time.Millisecond,
			Factor:   1.0,
			Jitter:   0.1,
		}, createFn); err != nil {
			log.Errorf("max retries for creating secret %s reached, err: %v", secretObj.SecretName, err)
			return ctrl.Result{}, err
		}
		err = r.patchSecretWithOwnerRef(ctx, secretObj.SecretName, req.Namespace, &spcPodStatus, r.Scheme)
		if err != nil {
			logger.Errorf("failed to set owner ref for secret, err: %+v", err)
			return ctrl.Result{RequeueAfter: 10 * time.Second}, err
		}
		success = append(success, secretObj.SecretName)
	}

	if len(success) > 0 {
		setStatusFn := func() (bool, error) {
			err := r.updateStatusOfSPC(ctx, spc.Name, spc.Namespace, success, r.Scheme)
			if err != nil {
				logger.Errorf("failed to update secret ref status in spc, err: %+v", err)
				return false, nil
			}
			return true, nil
		}
		if err := wait.ExponentialBackoff(wait.Backoff{
			Steps:    5,
			Duration: 1 * time.Millisecond,
			Factor:   1.0,
			Jitter:   0.1,
		}, setStatusFn); err != nil {
			log.Errorf("max retries for setting status reached, err: %v for spc: %s", err, spc.Name)
			return ctrl.Result{RequeueAfter: 10 * time.Second}, err
		}
	}

	logger.Info("Reconcile complete")
	return ctrl.Result{}, nil
}

func (r *SecretProviderClassPodStatusReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.SecretProviderClassPodStatus{}).
		Complete(r)
}

// createOrUpdateK8sSecret creates or updates a K8s secret with data from mounted files
// If a secret with the same name already exists in the namespace of the pod, it's updated.
func (r *SecretProviderClassPodStatusReconciler) createOrUpdateK8sSecret(ctx context.Context, name, namespace string, datamap map[string][]byte, secretType corev1.SecretType) error {
	secretKey := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Type: secretType,
	}

	err := r.Client.Get(ctx, secretKey, secret)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	// secret already exists
	if err == nil {
		return nil
	}
	secret.Data = datamap
	if err := r.Create(ctx, secret); err != nil {
		log.Errorf("error %v while creating K8s secret: %s, ns: %s", err, name, namespace)
		return err
	}
	log.Infof("created k8s secret: %s, ns: %s", name, namespace)
	return nil
}

func (r *SecretProviderClassPodStatusReconciler) patchSecretWithOwnerRef(ctx context.Context, name, namespace string, spcPodStatus *v1alpha1.SecretProviderClassPodStatus, scheme *runtime.Scheme) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	secretKey := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	err := r.Client.Get(ctx, secretKey, secret)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	old := secret.DeepCopyObject()
	err = controllerutil.SetOwnerReference(spcPodStatus, secret, scheme)
	if err != nil {
		return err
	}

	err = r.Patch(ctx, secret, client.MergeFrom(old))
	if err != nil {
		return err
	}
	return nil
}

func (r *SecretProviderClassPodStatusReconciler) updateStatusOfSPC(ctx context.Context, name, namespace string, secrets []string, scheme *runtime.Scheme) error {
	template := &v1alpha1.SecretProviderClass{}
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, template); err != nil {
		// If the template does not exist, we are done
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	var updateRequired bool
	m := make(map[string]bool)
	for _, s := range template.Status.SecretRef {
		m[s.Name] = s.Created
	}

	for _, secret := range secrets {
		created, ok := m[secret]
		if ok && created {
			continue
		}
		updateRequired = true
		template.Status.SecretRef = append(template.Status.SecretRef, &v1alpha1.SecretRefStatus{Name: secret, Created: true})
	}
	if !updateRequired {
		log.Info("secret status is already up-to date for spc")
		return nil
	}
	return r.Client.Update(ctx, template)
}
