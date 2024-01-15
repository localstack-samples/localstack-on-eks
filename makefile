SHELL := /bin/bash

-include .env
-include .env-local

.PHONY: \
    aws-setup-cluster \
    aws-cleanup-cluster \
    local-setup-cluster \
    local-cleanup-cluster \
    patch-coredns \
	deploy-setup \
	deploy-localstack \
	deploy-cleanup \
	exec-ssh-devpod

######################
# Solution 1 targets #
######################

aws-setup-cluster:
	eksctl create cluster --name $(CLUSTER_NAME) --region $(CLUSTER_REGION) --version 1.28 --fargate;
	kubectl create namespace ls$(NS_NUM);
	eksctl create fargateprofile \
		--cluster $(CLUSTER_NAME) \
		--name ls-fargate-profile$(NS_NUM) \
		--namespace ls$(NS_NUM);

aws-cleanup-cluster:
	eksctl delete fargateprofile \
		--cluster $(CLUSTER_NAME) \
		--name ls-fargate-profile$(NS_NUM);
	eksctl delete cluster --name $(CLUSTER_NAME) --region $(CLUSTER_REGION);

######################
# Solution 2 targets #
######################

local-setup-cluster:
	eksctl anywhere create cluster -f clusters/eks-anywhere/$(CLUSTER_NAME).yaml -v 6;
	kubectl --kubeconfig="$(shell pwd)/$(CLUSTER_NAME)/$(CLUSTER_NAME)-eks-a-cluster.kubeconfig" create namespace ls$(NS_NUM);
	echo "Run: export KUBECONFIG=$(shell pwd)/$(CLUSTER_NAME)/$(CLUSTER_NAME)-eks-a-cluster.kubeconfig";

local-cleanup-cluster:
	eksctl anywhere delete cluster -f clusters/eks-anywhere/$(CLUSTER_NAME).yaml -v 6;
	rm -r $(CLUSTER_NAME);

###################################
# Solution 1 & Solution 2 targets #
###################################

patch-coredns:
	# Patch CoreDNS to forward requests to localstack
	kubectl get -n kube-system configmaps coredns -o yaml | \
		yq  '.data.Corefile = (.data.Corefile + "\nlocalstack$(NS_NUM):53 {\n    errors\n    cache 5\n    forward . 10.100.$(NS_NUM).53\n}")' | \
		yq 'del(.metadata.annotations, .metadata.resourceVersion, .metadata.uid, .metadata.creationTimestamp)' | \
		kubectl apply -f -;
	
	# Restart CoreDNS
	kubectl rollout restart -n kube-system deployment/coredns;

	# Add service to expose Localstack DNS
	envsubst < manifests/coredns/service.template.yaml | kubectl apply -f -;

deploy-setup:
	export NODE_PORT=$(shell expr 31566 + ${NS_NUM})
	envsubst < charts/localstack/values.template.yaml > charts/localstack/values.yaml;
	envsubst < manifests/devxpod/deployment-template.yaml > manifests/devxpod/deployment-gen.yaml;
	helm repo add localstack-charts https://localstack.github.io/helm-charts;

deploy-localstack:
	$(MAKE) deploy-setup
	helm install localstack localstack-charts/localstack -f charts/localstack/values.yaml --namespace ls$(NS_NUM);
	kubectl apply -f manifests/devxpod/deployment-gen.yaml;

exec-ssh-devpod:
	DEV_POD_NAME=$(shell kubectl get pods -l app=devxpod -n ls$(NS_NUM) -o jsonpath="{.items[0].metadata.name}"); \
	kubectl exec -it $$DEV_POD_NAME -n ls$(NS_NUM) -- /bin/bash;

deploy-cleanup:
	helm uninstall localstack --namespace ls$(NS_NUM)
	kubectl delete namespace ls$(NS_NUM)

