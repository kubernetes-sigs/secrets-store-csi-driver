// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package controllers

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/secrets-store-csi-driver/apis/v1alpha1"
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
	logger := log.WithFields(log.Fields{"secretproviderclasspodstatus": req.NamespacedName})
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

	// reconcile delete
	// TODO (aramase)  what to do on delete of the spc pod status object
	// What action to take on the spc for this
	if !spcPodStatus.GetDeletionTimestamp().IsZero() {
		logger.Info("reconcile delete complete")
	}

	// targetPath := spcPodStatus.Status.TargetPath
	spcName := spcPodStatus.Status.SecretProviderClassName

	// recreating client here to prevent reading from cache
	c, err := getClient(r.Scheme)
	if err != nil {
		return ctrl.Result{}, err
	}

	var spc v1alpha1.SecretProviderClass

	// get the secret provider class object
	if err := c.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: spcName}, &spc); err != nil {
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

	return ctrl.Result{}, nil
}

func (r *SecretProviderClassPodStatusReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.SecretProviderClassPodStatus{}).
		Complete(r)
}

// getClient returns client.Client
func getClient(scheme *runtime.Scheme) (client.Client, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}
	c, err := client.New(cfg, client.Options{Scheme: scheme, Mapper: nil})
	if err != nil {
		return nil, err
	}
	return c, nil
}
