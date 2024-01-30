package controller

import (
	"context"
	"errors"
	"strings"

	lscv1alpha1 "github.com/localstack-samples/localstack-on-eks/pkg/crds/api/v1alpha1"
	"github.com/localstack-samples/localstack-on-eks/pkg/crds/internal/pointers"
	"github.com/localstack-samples/localstack-on-eks/pkg/crds/internal/providers/dns"
	kapps "k8s.io/api/apps/v1"
	kcore "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	kmeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// HELPER FUNCTIONS

func generateDomainName(localstackName string, localstackNamespace string) string {
	return "localstack-" + localstackName + "." + localstackNamespace
}

// GETTER HELPERS

func (r *LocalstackReconciler) getLocalstackDeployment(ctx context.Context, localstack *lscv1alpha1.Localstack) (*kapps.Deployment, error) {
	deployment := kapps.Deployment{}
	deploymentName := types.NamespacedName{
		Namespace: localstack.Namespace,
		Name:      "localstack-" + localstack.Name,
	}
	if err := r.Get(ctx, deploymentName, &deployment); err != nil {
		if !kerrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &deployment, nil
}

func (r *LocalstackReconciler) getGdcEnvDeployment(ctx context.Context, localstack *lscv1alpha1.Localstack) (*kapps.Deployment, error) {
	deployment := kapps.Deployment{}
	deploymentName := types.NamespacedName{
		Namespace: localstack.Namespace,
		Name:      "gdc-env-" + localstack.Name,
	}
	if err := r.Get(ctx, deploymentName, &deployment); err != nil {
		if !kerrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &deployment, nil
}

func (r *LocalstackReconciler) getLocalstackService(ctx context.Context, localstack *lscv1alpha1.Localstack) (*kcore.Service, error) {
	service := kcore.Service{}
	serviceName := types.NamespacedName{
		Namespace: localstack.Namespace,
		Name:      "localstack-" + localstack.Name,
	}
	if err := r.Get(ctx, serviceName, &service); err != nil {
		if !kerrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &service, nil
}

func (r *LocalstackReconciler) getLocalstackServiceIPAddress(ctx context.Context, localstack *lscv1alpha1.Localstack) (*string, error) {
	service, err := r.getLocalstackService(ctx, localstack)
	if err != nil {
		return nil, err
	}
	return &service.Spec.ClusterIP, nil
}

func (r *LocalstackReconciler) isDnsConfigured(ctx context.Context, localstack *lscv1alpha1.Localstack) (bool, error) {
	configmap := kcore.ConfigMap{}
	configmapName := types.NamespacedName{
		Namespace: localstack.Spec.DnsConfigNamespace,
		Name:      localstack.Spec.DnsConfigName,
	}
	if err := r.Get(ctx, configmapName, &configmap); err != nil {
		if !kerrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	configmapData := configmap.Data
	corefile, ok := configmapData["Corefile"]
	if !ok {
		return false, errors.New("corefile not found in configmap")
	}

	parser := &dns.DefaultCorefileParser{}
	config, err := parser.Unmarshal(corefile)
	if err != nil {
		return false, err
	}

	for _, name := range config.GetDirectiveNames() {
		if !strings.Contains(name, ":") {
			continue
		}
		domain_name := strings.Split(name, ":")[0]
		if generateDomainName(localstack.Name, localstack.Namespace) == domain_name {
			return true, nil
		}
	}

	return false, nil
}

// CREATE/UPDATE HELPERS

func (r *LocalstackReconciler) updateLocalstackStatus(
	ctx context.Context,
	localstack *lscv1alpha1.Localstack,
	deployment *kapps.Deployment,
	service *kcore.Service,
	gdcEnvDeployment *kapps.Deployment,
	dnsConfigured bool,
) error {
	if localstack == nil {
		return errors.New("unexpected localstack set to nil")
	}

	// check if deployment is ready
	readyLocalstack := false
	if deployment != nil {
		readyLocalstack = deployment.Status.ReadyReplicas == 1
	}

	// check if gdc-env deployment is ready
	readyDev := false
	if gdcEnvDeployment != nil {
		readyDev = gdcEnvDeployment.Status.ReadyReplicas == 1
	}

	// get localstack service IP address
	var ipAddress *string = nil
	if service != nil {
		ipAddress = &service.Spec.ClusterIP
	}

	// get localstack DNS address
	var dnsAddress *string = nil
	if dnsConfigured {
		dns := generateDomainName(localstack.Name, localstack.Namespace)
		dnsAddress = &dns
	}

	// generate localstack status
	localstack.Status = lscv1alpha1.LocalstackStatus{
		ReadyLocalstack: readyLocalstack,
		ReadyDev:        readyDev,
		IP:              ipAddress,
		DNS:             dnsAddress,
	}

	// update localstack status
	if err := r.Status().Update(ctx, localstack); err != nil {
		return err
	}
	return nil
}

func (r *LocalstackReconciler) createOrUpdateLocalstackDeployment(ctx context.Context, localstack *lscv1alpha1.Localstack) (controllerutil.OperationResult, error) {
	deployment := kapps.Deployment{
		ObjectMeta: kmeta.ObjectMeta{
			Namespace: localstack.Namespace,
			Name:      "localstack-" + localstack.Name,
		},
	}

	if localstack.Spec.LocalstackInstanceSpec.Resources == nil {
		localstack.Spec.LocalstackInstanceSpec.Resources = &kcore.ResourceRequirements{}
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, &deployment, func() error {
		deployment.Spec = kapps.DeploymentSpec{
			Replicas: pointers.Int32(1),
			Selector: &kmeta.LabelSelector{
				MatchLabels: map[string]string{
					"app": "localstack-" + localstack.Name,
				},
			},
			Template: kcore.PodTemplateSpec{
				ObjectMeta: kmeta.ObjectMeta{
					Labels: map[string]string{
						"app": "localstack-" + localstack.Name,
					},
				},
				Spec: kcore.PodSpec{
					Containers: []kcore.Container{
						{
							Name:           "localstack",
							Image:          localstack.Spec.LocalstackInstanceSpec.Image,
							Resources:      *localstack.Spec.LocalstackInstanceSpec.Resources,
							ReadinessProbe: localstack.Spec.LocalstackInstanceSpec.ReadinessProbe,
							LivenessProbe:  localstack.Spec.LocalstackInstanceSpec.LivenessProbe,
							Ports: []kcore.ContainerPort{
								{
									Name:          "localstack",
									ContainerPort: 4566,
								},
							},
							Env: []kcore.EnvVar{
								{
									Name: "SERVICES",
								},
								{
									Name:  "DEFAULT_REGION",
									Value: "us-east-1",
								},
							},
							EnvFrom: []kcore.EnvFromSource{
								{
									SecretRef: localstack.Spec.LocalstackInstanceSpec.AuthTokenSecretRef,
								},
							},
						},
					},
				},
			},
		}

		if err := ctrl.SetControllerReference(localstack, &deployment, r.Scheme); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return op, err
	}

	return op, nil
}

func (r *LocalstackReconciler) createOrUpdateLocalstackService(ctx context.Context, localstack *lscv1alpha1.Localstack) (controllerutil.OperationResult, error) {
	deployment := kapps.Deployment{
		ObjectMeta: kmeta.ObjectMeta{
			Namespace: localstack.Namespace,
			Name:      "localstack-" + localstack.Name,
		},
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, &deployment, func() error {
		deployment.Spec = kapps.DeploymentSpec{}

		if err := ctrl.SetControllerReference(localstack, &deployment, r.Scheme); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return op, err
	}

	return op, nil
}

func (r *LocalstackReconciler) createOrUpdateGdcEnvDeployment(ctx context.Context, localstack *lscv1alpha1.Localstack) (controllerutil.OperationResult, error) {
	deployment := kapps.Deployment{
		ObjectMeta: kmeta.ObjectMeta{
			Namespace: localstack.Namespace,
			Name:      "localstack-" + localstack.Name,
		},
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, &deployment, func() error {
		deployment.Spec = kapps.DeploymentSpec{}

		if err := ctrl.SetControllerReference(localstack, &deployment, r.Scheme); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return op, err
	}

	return op, nil
}

func (r *LocalstackReconciler) updateDnsConfig(ctx context.Context, localstack *lscv1alpha1.Localstack, localstackIPAddress string) (controllerutil.OperationResult, error) {
	deployment := kapps.Deployment{
		ObjectMeta: kmeta.ObjectMeta{
			Namespace: localstack.Namespace,
			Name:      "localstack-" + localstack.Name,
		},
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, &deployment, func() error {
		deployment.Spec = kapps.DeploymentSpec{}

		if err := ctrl.SetControllerReference(localstack, &deployment, r.Scheme); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return op, err
	}

	return op, nil
}
