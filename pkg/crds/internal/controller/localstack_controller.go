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

package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	lscv1alpha1 "github.com/localstack-samples/localstack-on-eks/pkg/crds/api/v1alpha1"
	kapps "k8s.io/api/apps/v1"
	kcore "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
)

// LocalstackReconciler reconciles a Localstack object
type LocalstackReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Log logr.Logger
}

//+kubebuilder:rbac:groups=localstack.cloud.localstack.cloud,resources=localstacks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=localstack.cloud.localstack.cloud,resources=localstacks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=localstack.cloud.localstack.cloud,resources=localstacks/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Localstack object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *LocalstackReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues(
		map[string]string{
			"localstack": req.NamespacedName.String(),
		},
	)

	// Step 1: get resource from request
	localstack := lscv1alpha1.Localstack{}
	log.V(1).Info("retrieving localstack resource")
	if err := r.Get(ctx, req.NamespacedName, &localstack); err != nil {
		if !kerrors.IsNotFound(err) {
			log.Error(err, "failed to retrieve localstack resource")
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LocalstackReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&lscv1alpha1.Localstack{}).
		Owns(&kapps.Deployment{}).
		Owns(&kcore.Service{}).
		Complete(r)
}
