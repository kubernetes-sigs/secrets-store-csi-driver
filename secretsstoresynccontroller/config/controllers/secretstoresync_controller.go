/*
Copyright 2023 The Kubernetes Authors.

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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	//secretsyncsv1alpha1 "github.com/secrets-store-sync/api/v1alpha1"
)

// SecretSyncReconciler reconciles a SecretSync object
type SecretSyncReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=secrets-store.sync.x-k8s.io,resources=secretsyncs,verbs=get;list;watch
//+kubebuilder:rbac:groups=secrets-store.sync.x-k8s.io,resources=secretsyncs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=create;patch
//+kubebuilder:rbac:groups="",resources="serviceaccounts/token",verbs=create
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=secrets-store.csi.x-k8s.io,resources=secretproviderclasses,verbs=get;list;watch

func (r *SecretSyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SecretSyncReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		//For(&secretsyncsv1alpha1.SecretSync{}).
		Complete(r)
}
