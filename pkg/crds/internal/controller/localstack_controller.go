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

	ss "github.com/localstack-samples/localstack-on-eks/pkg/crds/internal/strings"
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
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

// Add RBAC for CoreDNS configmap/deployment.
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;update;patch;delete;namespace=kube-system
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update;patch;delete;namespace=kube-system

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

	// Step 1: get localstack resource from request
	localstack := lscv1alpha1.Localstack{}
	log.V(1).Info("retrieving localstack resource")
	if err := r.Get(ctx, req.NamespacedName, &localstack); err != nil {
		if !kerrors.IsNotFound(err) {
			log.Error(err, "failed to retrieve localstack resource")
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Add finalizer for DNS ConfigMap
	finalizerName := "coredns.finalizer.localstack.cloud"
	if localstack.ObjectMeta.DeletionTimestamp.IsZero() {
		if !ss.ContainsString(localstack.ObjectMeta.Finalizers, finalizerName) {
			log.V(1).Info("Adding DNS config finalizer for the Localstack")
			localstack.ObjectMeta.Finalizers = append(localstack.ObjectMeta.Finalizers, finalizerName)
			if err := r.Update(ctx, &localstack); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if ss.ContainsString(localstack.ObjectMeta.Finalizers, finalizerName) {
			// run finalization logic for dnsConfigMap. If it fails, don't remove the finalizer so
			// we can retry during the next reconcile.
			if _, err := r.finalizeDnsConfig(ctx, &localstack); err != nil {
				return ctrl.Result{}, err
			}

			// remove the finalizer from the list and update it.
			log.V(1).Info("Removing finalizer for the Localstack")
			localstack.ObjectMeta.Finalizers = ss.RemoveString(localstack.ObjectMeta.Finalizers, finalizerName)
			if err := r.Update(ctx, &localstack); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	// Step 2: get localstack deployment
	log.V(1).Info("retrieving localstack deployment")
	deployment, err := r.getLocalstackDeployment(ctx, &localstack)
	if err != nil {
		log.Error(err, "failed to retrieve localstack deployment")
		return ctrl.Result{}, err
	}

	// Step 3: get localstack service
	log.V(1).Info("retrieving localstack service")
	service, err := r.getLocalstackService(ctx, &localstack)
	if err != nil {
		log.Error(err, "failed to retrieve localstack service")
		return ctrl.Result{}, err
	}

	// Step 4: get gdc-env deployment
	log.V(1).Info("retrieving gdc-env deployment")
	gdcEnvDeployment, err := r.getGdcEnvDeployment(ctx, &localstack)
	if err != nil {
		log.Error(err, "failed to retrieve gdc-env deployment")
		return ctrl.Result{}, err
	}

	// Step 5: check if DNS is configured
	log.V(1).Info("checking if DNS is configured")
	dnsConfigured, err := r.isDnsConfigured(ctx, &localstack)
	if err != nil {
		log.Error(err, "failed to check if DNS is configured")
		return ctrl.Result{}, err
	}
	log.V(1).Info("DNS is configured", "dnsConfigured", dnsConfigured)

	// Step 6: update localstack status
	log.V(1).Info("updating localstack status")
	if err := r.updateLocalstackStatus(ctx, &localstack, deployment, service, gdcEnvDeployment, dnsConfigured); err != nil {
		log.Error(err, "failed to update localstack status")
		return ctrl.Result{}, err
	}

	// Step 7: create/update gdc env deployment if specced
	if localstack.Spec.GDCEnvSpec != nil {
		log.V(1).Info("creating/updating gdc-env deployment")
		if _, err := r.createOrUpdateGdcEnvDeployment(ctx, &localstack); err != nil {
			log.Error(err, "failed to create/update gdc-env deployment")
			return ctrl.Result{}, err
		}
	}

	// Step 8: create/update localstack service
	log.V(1).Info("creating/updating localstack service")
	if _, err := r.createOrUpdateLocalstackService(ctx, &localstack); err != nil {
		log.Error(err, "failed to create/update localstack service")
		return ctrl.Result{}, err
	}

	// Step 9: retrieve service IP address if it exists
	log.V(1).Info("retrieving service IP address")
	serviceIP, err := r.getLocalstackServiceIPAddress(ctx, &localstack)
	if err != nil {
		log.Error(err, "failed to retrieve service IP address")
		return ctrl.Result{}, err
	}
	if serviceIP == nil {
		log.V(1).Info("service IP address is nil; wait for next reconcile loop")
		return ctrl.Result{}, nil
	}
	log.V(1).Info("service IP address is not nil")

	// Step 10: create/update localstack deployment
	log.V(1).Info("creating/updating localstack deployment")
	if _, err := r.createOrUpdateLocalstackDeployment(ctx, &localstack, *serviceIP); err != nil {
		log.Error(err, "failed to create/update localstack deployment")
		return ctrl.Result{}, err
	}

	// Step 11: update DNS config
	log.V(1).Info("updating DNS config")
	if _, err := r.updateDnsConfig(ctx, &localstack, *serviceIP); err != nil {
		log.Error(err, "failed to create/update DNS config")
		return ctrl.Result{}, err
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
