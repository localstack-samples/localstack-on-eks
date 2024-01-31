package controller

import (
	"context"
	"errors"
	"strings"

	lscv1alpha1 "github.com/localstack-samples/localstack-on-eks/pkg/crds/api/v1alpha1"
	"github.com/localstack-samples/localstack-on-eks/pkg/crds/internal/pointers"
	"github.com/localstack-samples/localstack-on-eks/pkg/crds/internal/providers/dns"
	ss "github.com/localstack-samples/localstack-on-eks/pkg/crds/internal/strings"
	"github.com/onsi/ginkgo/v2/config"
	kapps "k8s.io/api/apps/v1"
	kcore "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	kresource "k8s.io/apimachinery/pkg/api/resource"
	kmeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// CONSTANTS

const (
	LOCALSTACK_START_PORT = 4510
	LOCALSTACK_END_PORT   = 4560
)

// HELPER FUNCTIONS

func generateDomainName(localstackName string, localstackNamespace string) string {
	return "localstack-" + localstackName + "." + localstackNamespace
}

type EditCoreConfigFunc func(*dns.CoreConfig)
func readAndUpdateCoreDns(localstack *lscv1alpha1.Localstack, fn EditCoreConfigFunc) (controllerutil.OperationResult, error) {
	// Step 1: retrieve configmap
	configmap := kcore.ConfigMap{}
	configmapName := types.NamespacedName{
		Namespace: localstack.Spec.DnsConfigNamespace,
		Name:      localstack.Spec.DnsConfigName,
	}
	if err := r.Get(ctx, configmapName, &configmap); err != nil {
		return controllerutil.OperationResultNone, err
	}

	// Step 2: parse configmap data
	configmapData := configmap.Data
	corefile, ok := configmapData["Corefile"]
	if !ok {
		return controllerutil.OperationResultNone, errors.New("corefile not found in configmap")
	}

	parser := &dns.DefaultCorefileParser{}
	config, err := parser.Unmarshal(corefile)
	if err != nil {
		return controllerutil.OperationResultNone, err
	}
 
	// Step 3: edit configmap's Corefile data
	if err := fn(&config); err != nil {
		return controllerutil.OperationResultNone, err
	}

	// Step 4: update configmap
	corefile, err = parser.Marshal(config)
	if err != nil {
		return controllerutil.OperationResultNone, err
	}

	configmap.Data["Corefile"] = corefile
	if err := r.Update(ctx, &configmap); err != nil {
		return controllerutil.OperationResultNone, err
	}

	// Step 5: rollout CoreDNS deployment
	deployment := kapps.Deployment{}
	deploymentName := types.NamespacedName{
		Namespace: "kube-system",
		Name:      "coredns",
	}
	if err := r.Get(ctx, deploymentName, &deployment); err != nil {
		return controllerutil.OperationResultNone, err
	}

	// Step 6: update CoreDNS deployment
	deployment.Spec.Template.ObjectMeta.Annotations["kubectl.kubernetes.io/restartedAt"] = kmeta.NowMicro()
	if err := r.Update(ctx, &deployment); err != nil {
		return controllerutil.OperationResultNone, err
	}

	return controllerutil.OperationResultUpdated, nil
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

func (r *LocalstackReconciler) createOrUpdateLocalstackService(ctx context.Context, localstack *lscv1alpha1.Localstack) (controllerutil.OperationResult, error) {
	service := kcore.Service{
		ObjectMeta: kmeta.ObjectMeta{
			Namespace: localstack.Namespace,
			Name:      "localstack-" + localstack.Name,
		},
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, &service, func() error {
		service.Spec = r.desiredLocalstackService(localstack)

		if err := ctrl.SetControllerReference(localstack, &service, r.Scheme); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return op, err
	}

	return op, nil
}

func (r *LocalstackReconciler) createOrUpdateLocalstackDeployment(ctx context.Context, localstack *lscv1alpha1.Localstack, dnsResolveIp string) (controllerutil.OperationResult, error) {
	deployment := kapps.Deployment{
		ObjectMeta: kmeta.ObjectMeta{
			Namespace: localstack.Namespace,
			Name:      "localstack-" + localstack.Name,
		},
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, &deployment, func() error {
		deployment.Spec = r.desiredLocalstackDeployment(localstack, dnsResolveIp)

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

// Retrieve configmap's `Corefile` data from DNS config and parse it.
// After parsing it, we can remove the domain name from the configmap
// and update it.
// Finally, rollout the CoreDNS deployment to apply the new configmap.
func (r *LocalstackReconciler) finalizeDnsConfig(ctx context.Context, localstack *lscv1alpha1.Localstack) (controllerutil.OperationResult, error) {
	result, err := readAndUpdateCoreDns(localstack, func(config *dns.CoreConfig) {
		domainName := generateDomainName(localstack.Name, localstack.Namespace)
		directiveName := domainName + ":53"
		config.RemoveDirective(directiveName)
	})
	if err != nil {
		return controllerutil.OperationResultNone, err
	}
	return result, nil
}

// Retrieve configmap's `Corefile` data from DNS config and parse it.
// After parsing it, we can add the new domain name to the configmap
// and update it.
// Finally, rollout the CoreDNS deployment to apply the new configmap.
func (r *LocalstackReconciler) updateDnsConfig(ctx context.Context, localstack *lscv1alpha1.Localstack, dnsResolveIp string) (controllerutil.OperationResult, error) {
	result, err := readAndUpdateCoreDns(localstack, func(config *dns.CoreConfig) {
		domainName := generateDomainName(localstack.Name, localstack.Namespace)
		directiveName := domainName + ":53"
		directive := dns.Directive{
			Name: directiveName,
			Entries: []dns.Entry {
				{StrValue: "errors"},
				{StrValue: "cache 5"},
				{StrValue: "forward . " + dnsResolveIp},
			},
		}
		config.AddDirective(directive)
	})
	if err != nil {
		return controllerutil.OperationResultNone, err
	}
	return result, nil
}

// SPECS

func (r *LocalstackReconciler) desiredLocalstackService(localstack *lscv1alpha1.Localstack) kcore.ServiceSpec {
	startPort := LOCALSTACK_START_PORT
	endPort := LOCALSTACK_END_PORT
	servicePorts := []kcore.ServicePort{
		{Port: 53, Name: "dns-svc", Protocol: kcore.ProtocolTCP},
	}
	for i := startPort; i <= endPort; i++ {
		servicePorts = append(servicePorts, kcore.ServicePort{Port: int32(i), Name: "ext-svc-" + ss.IntToString(i), Protocol: kcore.ProtocolTCP})
	}
	return kcore.ServiceSpec{
		Selector: map[string]string{
			"app": "localstack-" + localstack.Name,
		},
		Ports: servicePorts,
		Type: kcore.ServiceTypeClusterIP,
	}
}

func (r *LocalstackReconciler) desiredLocalstackDeployment(localstack *lscv1alpha1.Localstack, dnsResolveIp string) kapps.DeploymentSpec {
	// Default resources requests
	resources := kcore.ResourceRequirements{
		Requests: kcore.ResourceList{
			kcore.ResourceCPU:    kresource.MustParse("100m"),
			kcore.ResourceMemory: kresource.MustParse("128Mi"),
		},
	}
	if localstack.Spec.LocalstackInstanceSpec.Resources != nil {
		resources = *localstack.Spec.LocalstackInstanceSpec.Resources
	}

	// Default readiness probe
	readinessProbe := &kcore.Probe{
		ProbeHandler: kcore.ProbeHandler{
			HTTPGet: &kcore.HTTPGetAction{
				Path:   "/_localstack/health",
				Port:   intstr.FromString("edge"),
				Scheme: kcore.URISchemeHTTP,
			},
		},
		InitialDelaySeconds: 15,
		PeriodSeconds:       10,
		TimeoutSeconds:      1,
		SuccessThreshold:    1,
		FailureThreshold:    3,
	}
	if localstack.Spec.LocalstackInstanceSpec.ReadinessProbe != nil {
		readinessProbe = localstack.Spec.LocalstackInstanceSpec.ReadinessProbe
	}

	// Default liveness probe
	livenessProbe := &kcore.Probe{
		ProbeHandler: kcore.ProbeHandler{
			HTTPGet: &kcore.HTTPGetAction{
				Path:   "/_localstack/health",
				Port:   intstr.FromString("edge"),
				Scheme: kcore.URISchemeHTTP,
			},
		},
		InitialDelaySeconds: 15,
		PeriodSeconds:       10,
		TimeoutSeconds:      1,
		SuccessThreshold:    1,
		FailureThreshold:    3,
	}
	if localstack.Spec.LocalstackInstanceSpec.LivenessProbe != nil {
		livenessProbe = localstack.Spec.LocalstackInstanceSpec.LivenessProbe
	}

	startPort := LOCALSTACK_START_PORT
	endPort := LOCALSTACK_END_PORT
	envVars := []kcore.EnvVar{
		{Name: "DEBUG", Value: ss.BoolToString(localstack.Spec.LocalstackInstanceSpec.Debug)},
		{Name: "EXTERNAL_SERVICE_PORTS_START", Value: ss.IntToString(startPort)},
		{Name: "EXTERNAL_SERVICE_PORTS_END", Value: ss.IntToString(endPort)},
		{Name: "LOCALSTACK_K8S_SERVICE_NAME", Value: "localstack-" + localstack.Name},
		{Name: "LOCALSTACK_K8S_NAMESPACE", ValueFrom: &kcore.EnvVarSource{FieldRef: &kcore.ObjectFieldSelector{APIVersion: "v1", FieldPath: "metadata.namespace"}}},
		{Name: "LAMBDA_RUNTIME_EXECUTOR", Value: "kubernetes"},
		{Name: "LAMBDA_K8S_IMAGE_PREFIX", Value: "localstack/lambda-"},
		{Name: "OVERRIDE_IN_DOCKER", Value: "1"},
		{Name: "GATEWAY_LISTEN", Value: "0.0.0.0:4566"},
		{Name: "DNS_RESOLVE_IP", Value: dnsResolveIp},
		{Name: "LOCALSTACK_HOST", Value: generateDomainName(localstack.Name, localstack.Namespace) + ":4566"},
	}
	if localstack.Spec.LocalstackInstanceSpec.LambdaEnvironmentTimeout != nil {
		lambdaRuntimeEnvironmentTimeout := int(localstack.Spec.LocalstackInstanceSpec.LambdaEnvironmentTimeout.Duration.Seconds())
		lambdaRuntimeEnvironmentTimeoutStr := ss.IntToString(lambdaRuntimeEnvironmentTimeout)
		envVars = append(envVars, kcore.EnvVar{Name: "LAMBDA_RUNTIME_ENVIRONMENT_TIMEOUT", Value: lambdaRuntimeEnvironmentTimeoutStr})
	if localstack.Spec.LocalstackInstanceSpec.AuthToken != nil {
		envVars = append(envVars, kcore.EnvVar{Name: "LOCALSTACK_AUTH_TOKEN", Value: *localstack.Spec.LocalstackInstanceSpec.AuthToken})
	}

	containerPorts := []kcore.ContainerPort{
		{ContainerPort: 53, Name: "dns-svc", Protocol: kcore.ProtocolTCP},
	}
	for i := startPort; i <= endPort; i++ {
		containerPorts = append(containerPorts, kcore.ContainerPort{ContainerPort: int32(i), Name: "ext-svc-" + ss.IntToString(i), Protocol: kcore.ProtocolTCP})
	}

	return kapps.DeploymentSpec{
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
						Name:  "localstack",
						Image: localstack.Spec.LocalstackInstanceSpec.Image,
						Command: []string{
							"/bin/bash",
							"-c",
							"echo 'ulimit -Sn 32767' >> /root/.bashrc && echo 'ulimit -Su 16383' >> /root/.bashrc && docker-entrypoint.sh",
						},
						Env: envVars,
						Ports: containerPorts,
						ImagePullPolicy: kcore.PullIfNotPresent,
						LivenessProbe:   livenessProbe,
						ReadinessProbe:  readinessProbe,
						Resources:       resources,
					},
				},
			},
		},
	}
}
