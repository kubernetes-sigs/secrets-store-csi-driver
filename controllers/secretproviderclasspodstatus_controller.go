/*
Copyright 2020 The Kubernetes Authors.

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
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
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
	certType          = "CERTIFICATE"
	privateKeyType    = "RSA PRIVATE KEY"
	internalNodeLabel = "internal.secrets-store.csi.k8s.io/node-name"
)

// SecretProviderClassPodStatusReconciler reconciles a SecretProviderClassPodStatus object
type SecretProviderClassPodStatusReconciler struct {
	client.Client
	Log    *log.Logger
	Scheme *runtime.Scheme
	NodeID string
	Reader client.Reader
	Writer client.Writer
}

// +kubebuilder:rbac:groups=secrets-store.csi.x-k8s.io,resources=secretproviderclasspodstatuses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=secrets-store.csi.x-k8s.io,resources=secretproviderclasspodstatuses/status,verbs=get;update;patch

func (r *SecretProviderClassPodStatusReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	logger := log.WithFields(log.Fields{"secretproviderclasspodstatus": req.NamespacedName, "node": r.NodeID})
	logger.Info("reconcile started")

	var spcPodStatus v1alpha1.SecretProviderClassPodStatus
	if err := r.Get(ctx, req.NamespacedName, &spcPodStatus); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Errorf("failed to get spc pod status, err: %+v", err)
		return ctrl.Result{}, err
	}

	// reconcile delete
	if !spcPodStatus.GetDeletionTimestamp().IsZero() {
		logger.Infof("reconcile complete")
		return ctrl.Result{}, nil
	}

	node, ok := spcPodStatus.GetLabels()[internalNodeLabel]
	if !ok {
		logger.Info("node label not found, ignoring this spc pod status")
		return ctrl.Result{}, nil
	}
	if !strings.EqualFold(node, r.NodeID) {
		logger.Infof("ignoring as spc pod status belongs to node %s", node)
		return ctrl.Result{}, nil
	}

	spcName := spcPodStatus.Status.SecretProviderClassName
	spc := &v1alpha1.SecretProviderClass{}
	if err := r.Reader.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: spcName}, spc); err != nil {
		logger.Errorf("failed to get spc %s, err: %+v", spcName, err)
		if apierrors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		return ctrl.Result{}, err
	}

	if len(spc.Spec.SecretObjects) == 0 {
		logger.Infof("no secret objects defined for spc, nothing to reconcile")
		return ctrl.Result{}, nil
	}

	files, err := getMountedFiles(spcPodStatus.Status.TargetPath)
	if err != nil {
		logger.Errorf("failed to get mounted files, err: %+v", err)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}
	errs := make([]error, 0)
	for idx, secretObj := range spc.Spec.SecretObjects {
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
		exists, err := r.secretExists(ctx, secretObj.SecretName, req.Namespace)
		if err != nil {
			logger.Errorf("failed to check if secret %s exists, err: %+v", secretObj.SecretName, err)
			errs = append(errs, fmt.Errorf("failed to check if secret %s exists, err: %+v", secretObj.SecretName, err))
			continue
		}
		funcs := []func() (bool, error){}

		if !exists {
			secretType := getSecretType(secretObj.Type)
			datamap := make(map[string][]byte)

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
				logger.Infof("file matching objectName %s found for key %s, secret %s", data.ObjectName, data.Key, secretObj.SecretName)
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
				if err := r.createK8sSecret(ctx, secretObj.SecretName, req.Namespace, datamap, secretType); err != nil {
					logger.Errorf("failed createK8sSecret, err: %v for secret: %s", err, secretObj.SecretName)
					return false, nil
				}
				return true, nil
			}
			funcs = append(funcs, createFn)
		}

		// patch the secret with the owner reference
		patchFn := func() (bool, error) {
			if err := r.patchSecretWithOwnerRef(ctx, secretObj.SecretName, req.Namespace, &spcPodStatus); err != nil {
				logger.Errorf("failed to set owner ref for secret, err: %+v", err)
				return false, nil
			}
			return true, nil
		}

		funcs = append(funcs, patchFn)
		for _, f := range funcs {
			if err := wait.ExponentialBackoff(wait.Backoff{
				Steps:    5,
				Duration: 1 * time.Millisecond,
				Factor:   1.0,
				Jitter:   0.1,
			}, f); err != nil {
				return ctrl.Result{RequeueAfter: 5 * time.Second}, err
			}
		}
	}

	if len(errs) > 0 {
		return ctrl.Result{Requeue: true}, nil
	}

	logger.Info("reconcile complete")
	return ctrl.Result{}, nil
}

func (r *SecretProviderClassPodStatusReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.SecretProviderClassPodStatus{}).
		Complete(r)
}

// createK8sSecret creates K8s secret with data from mounted files
// If a secret with the same name already exists in the namespace of the pod, the error is nil.
func (r *SecretProviderClassPodStatusReconciler) createK8sSecret(ctx context.Context, name, namespace string, datamap map[string][]byte, secretType corev1.SecretType) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Type: secretType,
		Data: datamap,
	}

	err := r.Create(ctx, secret)
	if err == nil {
		log.Infof("created k8s secret: %s/%s", namespace, name)
		return nil
	}
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func (r *SecretProviderClassPodStatusReconciler) patchSecretWithOwnerRef(ctx context.Context, name, namespace string, spcPodStatus *v1alpha1.SecretProviderClassPodStatus) error {
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
	err := r.Reader.Get(ctx, secretKey, secret)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	err = controllerutil.SetOwnerReference(spcPodStatus, secret, r.Scheme)
	if err != nil {
		return err
	}
	err = r.Writer.Update(ctx, secret)
	if err != nil {
		return err
	}
	return nil
}

func (r *SecretProviderClassPodStatusReconciler) secretExists(ctx context.Context, name, namespace string) (bool, error) {
	o := &v1.Secret{}
	secretKey := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	err := r.Client.Get(ctx, secretKey, o)
	if err == nil {
		return true, nil
	}
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	return false, err
}
