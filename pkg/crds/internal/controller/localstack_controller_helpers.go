package controller

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	lscv1alpha1 "github.com/localstack-samples/localstack-on-eks/pkg/crds/api/v1alpha1"
	err "github.com/localstack-samples/localstack-on-eks/pkg/crds/internal/errors"
	"github.com/localstack-samples/localstack-on-eks/pkg/crds/internal/pointers"
	"github.com/localstack-samples/localstack-on-eks/pkg/crds/internal/providers/dns"
	ss "github.com/localstack-samples/localstack-on-eks/pkg/crds/internal/strings"
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

// HELPER FUNCTIONS

func generateDomainName(localstackName string, localstackNamespace string) string {
	return "localstack-" + localstackName + "." + localstackNamespace
}

type EditCoreConfigFunc func(*dns.CoreConfig) bool

func (r *LocalstackReconciler) readAndUpdateCoreDns(ctx context.Context, localstack *lscv1alpha1.Localstack, fn EditCoreConfigFunc) (*ctrl.Result, controllerutil.OperationResult, error) {
	// Step 1: retrieve configmap
	configmap := kcore.ConfigMap{}
	configmapName := types.NamespacedName{
		Namespace: localstack.Spec.DnsConfigNamespace,
		Name:      localstack.Spec.DnsConfigName,
	}
	if err := r.Get(ctx, configmapName, &configmap); err != nil {
		// check if "Operation cannot be fulfilled on deployments.apps \"coredns\": the object has been modified; please apply your changes to the latest version and try again"

		if kerrors.IsConflict(err) {
			// Object is invalid, possibly due to using stale UID, requeue the request
			r.Log.Error(err, "configmap is invalid")
			return &ctrl.Result{
				Requeue:      true,
				RequeueAfter: 5 * time.Second,
			}, controllerutil.OperationResultNone, nil
		}

		return nil, controllerutil.OperationResultNone, err
	}

	// Step 2: parse configmap data
	configmapData := configmap.Data
	corefile, ok := configmapData["Corefile"]
	if !ok {
		return nil, controllerutil.OperationResultNone, errors.New("corefile not found in configmap")
	}

	parser := &dns.DefaultCorefileParser{}
	config, err := parser.Unmarshal(corefile)
	if err != nil {
		return nil, controllerutil.OperationResultNone, err
	}

	// Step 3: edit configmap's Corefile data
	changed := fn(&config)

	// If no changes were made, return early
	if !changed {
		return nil, controllerutil.OperationResultNone, nil
	}

	// Step 4: update configmap
	corefile, err = parser.Marshal(config)
	if err != nil {
		return nil, controllerutil.OperationResultNone, err
	}

	configmap.Data["Corefile"] = corefile
	if err := r.Update(ctx, &configmap); err != nil {
		return nil, controllerutil.OperationResultNone, err
	}

	// Step 5: rollout CoreDNS deployment
	deployment := kapps.Deployment{}
	deploymentName := types.NamespacedName{
		Namespace: "kube-system",
		Name:      "coredns",
	}
	if err := r.Get(ctx, deploymentName, &deployment); err != nil {
		if kerrors.IsNotFound(err) || kerrors.IsInvalid(err) {
			// Object is not found, invalid, possibly due to using stale UID, requeue the request
			return &ctrl.Result{
				Requeue:      true,
				RequeueAfter: 5 * time.Second,
			}, controllerutil.OperationResultNone, nil
		}
		return nil, controllerutil.OperationResultNone, err
	}

	// Step 6: update CoreDNS deployment
	if deployment.Spec.Template.ObjectMeta.Annotations == nil {
		deployment.Spec.Template.ObjectMeta.Annotations = map[string]string{}
	}
	deployment.Spec.Template.ObjectMeta.Annotations["kubectl.kubernetes.io/restartedAt"] = kmeta.NowMicro().Time.String()
	if err := r.Update(ctx, &deployment); err != nil {
		if kerrors.IsConflict(err) {
			// Object is invalid, possibly due to using stale UID, requeue the request
			r.Log.Error(err, "coredns deployment is invalid")
			return &ctrl.Result{
				Requeue:      true,
				RequeueAfter: 5 * time.Second,
			}, controllerutil.OperationResultNone, nil
		}
		return nil, controllerutil.OperationResultNone, err
	}
	return nil, controllerutil.OperationResultUpdated, nil
}

// GETTER HELPERS

func (r *LocalstackReconciler) validateLocalstackResource(localstack *lscv1alpha1.Localstack) error {
	disallowedEnvVars := []string{
		"DEBUG",
		"EXTERNAL_SERVICE_PORTS_START",
		"EXTERNAL_SERVICE_PORTS_END",
		"LOCALSTACK_K8S_SERVICE_NAME",
		"LOCALSTACK_K8S_NAMESPACE",
		"LAMBDA_RUNTIME_EXECUTOR",
		"LAMBDA_K8S_IMAGE_PREFIX",
		"OVERRIDE_IN_DOCKER",
		"GATEWAY_LISTEN",
		"DNS_RESOLVE_IP",
		"LOCALSTACK_HOST",
		"LOCALSTACK_AUTH_TOKEN",
	}
	for _, envVar := range localstack.Spec.Env {
		if ss.ContainsString(disallowedEnvVars, envVar.Name) {
			return err.NewWithRecorder(r.Recorder, "disallowed environment variable: "+envVar.Name)
		}
	}

	return nil
}

func (r *LocalstackReconciler) getLocalstackDeployment(ctx context.Context, localstack *lscv1alpha1.Localstack) (*kapps.Deployment, error) {
	deployment := kapps.Deployment{}
	deploymentName := types.NamespacedName{
		Namespace: localstack.Namespace,
		Name:      "localstack-" + localstack.Name,
	}
	if err := r.Get(ctx, deploymentName, &deployment); err != nil {
		if kerrors.IsNotFound(err) {
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
		if kerrors.IsNotFound(err) {
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
	if service == nil {
		return nil, nil
	}
	if service.Spec.ClusterIP == "" {
		return nil, nil
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
		if kerrors.IsNotFound(err) {
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
		Ready: readyLocalstack,
		IP:    ipAddress,
		DNS:   dnsAddress,
	}

	// update localstack status
	if err := r.Status().Update(ctx, localstack); err != nil {
		return err
	}
	return nil
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

func (r *LocalstackReconciler) createOrUpdateLocalstackDeployment(ctx context.Context, localstack *lscv1alpha1.Localstack, dnsResolveIp string) (*ctrl.Result, controllerutil.OperationResult, error) {
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

	if kerrors.IsConflict(err) {
		// Object is invalid, possibly due to using stale UID, requeue the request
		return &ctrl.Result{
			Requeue:      true,
			RequeueAfter: 5 * time.Second,
		}, op, nil
	}

	return nil, op, err
}

// Retrieve configmap's `Corefile` data from DNS config and parse it.
// After parsing it, we can remove the domain name from the configmap
// and update it.
// Finally, rollout the CoreDNS deployment to apply the new configmap.
func (r *LocalstackReconciler) finalizeDnsConfig(ctx context.Context, localstack *lscv1alpha1.Localstack) (*ctrl.Result, controllerutil.OperationResult, error) {
	return r.readAndUpdateCoreDns(ctx, localstack, func(config *dns.CoreConfig) bool {
		domainName := generateDomainName(localstack.Name, localstack.Namespace)
		directiveName := domainName + ":53"
		changed := config.RemoveDirective(directiveName) > 0
		return changed
	})
}

// Retrieve configmap's `Corefile` data from DNS config and parse it.
// After parsing it, we can add the new domain name to the configmap
// and update it.
// Finally, rollout the CoreDNS deployment to apply the new configmap.
func (r *LocalstackReconciler) updateDnsConfig(ctx context.Context, localstack *lscv1alpha1.Localstack, dnsResolveIp string) (*ctrl.Result, controllerutil.OperationResult, error) {
	return r.readAndUpdateCoreDns(ctx, localstack, func(config *dns.CoreConfig) bool {
		domainName := generateDomainName(localstack.Name, localstack.Namespace)
		directiveName := domainName + ":53"
		directive := dns.Directive{
			Name: directiveName,
			Entries: []dns.DirectiveEntry{
				{StrValue: "errors"},
				{StrValue: "cache 5"},
				{StrValue: "forward . " + dnsResolveIp},
			},
		}
		changed := false
		if !config.HasDirective(directiveName) {
			config.AddDirective(directive)
			changed = true
		}
		if config.KeepUniqueDirectives() > 0 {
			changed = true
		}
		return changed
	})
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
	if endPort < LOCALSTACK_SERVER_PORT {
		servicePorts = append(servicePorts, kcore.ServicePort{Port: LOCALSTACK_SERVER_PORT, Name: "localstack-svc", Protocol: kcore.ProtocolTCP})
	}
	return kcore.ServiceSpec{
		Selector: map[string]string{
			"app": "localstack-" + localstack.Name,
		},
		Ports: servicePorts,
		Type:  kcore.ServiceTypeClusterIP,
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
	if localstack.Spec.Resources != nil {
		resources = *localstack.Spec.Resources
	}

	// Default readiness probe
	readinessProbe := &kcore.Probe{
		ProbeHandler: kcore.ProbeHandler{
			HTTPGet: &kcore.HTTPGetAction{
				Path:   "/_localstack/health",
				Port:   intstr.FromInt(LOCALSTACK_SERVER_PORT),
				Scheme: kcore.URISchemeHTTP,
			},
		},
		InitialDelaySeconds: 15,
		PeriodSeconds:       10,
		TimeoutSeconds:      1,
		SuccessThreshold:    1,
		FailureThreshold:    3,
	}
	if localstack.Spec.ReadinessProbe != nil {
		readinessProbe = localstack.Spec.ReadinessProbe
	}

	// Default liveness probe
	livenessProbe := &kcore.Probe{
		ProbeHandler: kcore.ProbeHandler{
			HTTPGet: &kcore.HTTPGetAction{
				Path:   "/_localstack/health",
				Port:   intstr.FromInt(LOCALSTACK_SERVER_PORT),
				Scheme: kcore.URISchemeHTTP,
			},
		},
		InitialDelaySeconds: 15,
		PeriodSeconds:       10,
		TimeoutSeconds:      1,
		SuccessThreshold:    1,
		FailureThreshold:    3,
	}
	if localstack.Spec.LivenessProbe != nil {
		livenessProbe = localstack.Spec.LivenessProbe
	}

	startPort := LOCALSTACK_START_PORT
	endPort := LOCALSTACK_END_PORT
	envVars := []kcore.EnvVar{
		{Name: "DEBUG", Value: ss.BoolToString(localstack.Spec.Debug)},
		{Name: "EXTERNAL_SERVICE_PORTS_START", Value: ss.IntToString(startPort)},
		{Name: "EXTERNAL_SERVICE_PORTS_END", Value: ss.IntToString(endPort)},
		{Name: "LOCALSTACK_K8S_SERVICE_NAME", Value: "localstack-" + localstack.Name},
		{Name: "LOCALSTACK_K8S_NAMESPACE", ValueFrom: &kcore.EnvVarSource{FieldRef: &kcore.ObjectFieldSelector{APIVersion: "v1", FieldPath: "metadata.namespace"}}},
		{Name: "LAMBDA_RUNTIME_EXECUTOR", Value: "kubernetes"},
		{Name: "LAMBDA_K8S_IMAGE_PREFIX", Value: "localstack/lambda-"},
		{Name: "OVERRIDE_IN_DOCKER", Value: "1"},
		{Name: "GATEWAY_LISTEN", Value: fmt.Sprintf("0.0.0.0:%d", LOCALSTACK_SERVER_PORT)},
		{Name: "DNS_RESOLVE_IP", Value: dnsResolveIp},
		{Name: "LOCALSTACK_HOST", Value: fmt.Sprintf("%s:%d", generateDomainName(localstack.Name, localstack.Namespace), LOCALSTACK_SERVER_PORT)},
	}
	if localstack.Spec.AuthToken != nil {
		envVars = append(envVars, kcore.EnvVar{Name: "LOCALSTACK_AUTH_TOKEN", Value: *localstack.Spec.AuthToken})
	}

	containerPorts := []kcore.ContainerPort{
		{ContainerPort: 53, Name: "dns-svc", Protocol: kcore.ProtocolTCP},
	}
	for i := startPort; i <= endPort; i++ {
		containerPorts = append(containerPorts, kcore.ContainerPort{ContainerPort: int32(i), Name: "ext-svc-" + ss.IntToString(i), Protocol: kcore.ProtocolTCP})
	}

	envVars = append(envVars, localstack.Spec.Env...)

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
						Image: localstack.Spec.Image,
						Command: []string{
							"/bin/bash",
							"-c",
							"echo 'ulimit -Sn 32767' >> /root/.bashrc && echo 'ulimit -Su 16383' >> /root/.bashrc && docker-entrypoint.sh",
						},
						Ports:           containerPorts,
						ImagePullPolicy: kcore.PullIfNotPresent,
						LivenessProbe:   livenessProbe,
						ReadinessProbe:  readinessProbe,
						Resources:       resources,
						EnvFrom:         localstack.Spec.EnvFrom,
						Env:             envVars,
					},
				},
				DNSPolicy: localstack.Spec.DNSPolicy,
			},
		},
	}
}
