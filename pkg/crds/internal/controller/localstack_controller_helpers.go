package controller

import (
	"context"
	"errors"
	"strings"

	lscv1alpha1 "github.com/localstack-samples/localstack-on-eks/pkg/crds/api/v1alpha1"
	"github.com/localstack-samples/localstack-on-eks/pkg/crds/internal/providers/dns"
	kapps "k8s.io/api/apps/v1"
	kcore "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

func generateDomainName(localstackName string, localstackNamespace string) string {
	return "localstack-" + localstackName + "." + localstackNamespace
}

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
