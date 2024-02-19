SHELL := /bin/bash

-include .env
-include .env-local

.PHONY: \
    aws-create-cluster \
    aws-delete-cluster \
    local-create-cluster \
    local-delete-cluster \
	create-namespace \
    patch-coredns \
	deploy-setup \
	deploy-localstack \
	deploy-cleanup \
	exec-devpod-interactive

######################
# Helper targets     #
######################

check-ls-num:
ifndef NS_NUM
	$(error NS_NUM is not set)
endif

check-cmd:
ifndef CMD
	$(error CMD is not set)
endif

######################
# Solution 1 targets #
######################

aws-create-cluster:
	envsubst < clusters/aws/$(CLUSTER_NAME).template.yaml > clusters/aws/$(CLUSTER_NAME).yaml;
	eksctl create cluster --config-file clusters/aws/$(CLUSTER_NAME).yaml;
	mkdir -p ~/.kube;
	mv ~/.kube/config ~/.kube/config.bak || true;
	cp "$(shell pwd)/$(CLUSTER_NAME)/$(CLUSTER_NAME)-eks-a-cluster.kubeconfig" ~/.kube/config;


aws-delete-cluster:
	eksctl delete cluster --name $(CLUSTER_NAME) --region $(CLUSTER_REGION);
	rm -r $(CLUSTER_NAME) eksa-cli-logs;

aws-create-fargate-profile: check-ls-num create-namespace
	eksctl create fargateprofile \
		--cluster $(CLUSTER_NAME) \
		--name ls-fargate-profile$(NS_NUM) \
		--namespace ls$(NS_NUM);

aws-delete-fargate-profile: check-ls-num delete-namespace
	eksctl delete fargateprofile \
		--cluster $(CLUSTER_NAME) \
		--name ls-fargate-profile$(NS_NUM);

######################
# Solution 2 targets #
######################

local-create-cluster:
	envsubst < clusters/eks-anywhere/$(CLUSTER_NAME).template.yaml > clusters/eks-anywhere/$(CLUSTER_NAME).yaml;
	eksctl anywhere create cluster -f clusters/eks-anywhere/$(CLUSTER_NAME).yaml -v 6;
	mkdir -p ~/.kube
	mv ~/.kube/config ~/.kube/config.bak || true
	cp "$(shell pwd)/$(CLUSTER_NAME)/$(CLUSTER_NAME)-eks-a-cluster.kubeconfig" ~/.kube/config

local-delete-cluster:
	eksctl anywhere delete cluster -f clusters/eks-anywhere/$(CLUSTER_NAME).yaml -v 6;
	rm -r $(CLUSTER_NAME) eksa-cli-logs;

###################################
# Solution 1 & Solution 2 targets #
###################################

create-namespace: check-ls-num
	kubectl create namespace ls$(NS_NUM) --dry-run=client -o yaml | kubectl apply -f -;

delete-namespace: check-ls-num
	kubectl delete namespace ls$(NS_NUM)  --ignore-not-found=true;

patch-coredns: check-ls-num
	# Patch CoreDNS to forward requests to localstack
	kubectl get -n kube-system configmaps coredns -o yaml | \
		yq  '.data.Corefile = (.data.Corefile + "\nlocalstack$(NS_NUM):53 {\n    errors\n    cache 5\n    forward . 10.100.$(NS_NUM).53\n}")' | \
		yq 'del(.metadata.annotations, .metadata.resourceVersion, .metadata.uid, .metadata.creationTimestamp)' | \
		kubectl apply -f -;
	
	# Restart CoreDNS
	kubectl rollout restart -n kube-system deployment/coredns;

	# Add service to expose Localstack DNS
	envsubst < manifests/coredns/service.template.yaml | kubectl apply -f -;

deploy-setup: check-ls-num
	export NODE_PORT=$(shell expr 31566 + ${NS_NUM})
	envsubst < charts/localstack/values.template.yaml > charts/localstack/values.yaml;
	envsubst < manifests/devxpod/deployment-template.yaml > manifests/devxpod/deployment-gen.yaml;
	helm repo add localstack-charts https://localstack.github.io/helm-charts;
	helm repo update localstack-charts;

deploy-localstack: check-ls-num
	$(MAKE) deploy-setup
	helm install localstack localstack-charts/localstack -f charts/localstack/values.yaml --namespace ls$(NS_NUM);
	kubectl apply -f manifests/devxpod/deployment-gen.yaml;

exec-devpod-interactive: check-ls-num
	DEV_POD_NAME=$(shell kubectl get pods -l app=devxpod -n ls$(NS_NUM) -o jsonpath="{.items[0].metadata.name}"); \
	kubectl exec -it $$DEV_POD_NAME -n ls$(NS_NUM) -- /bin/bash;

exec-devpod-noninteractive: check-ls-num check-cmd
	DEV_POD_NAME=$(shell kubectl get pods -l app=devxpod -n ls$(NS_NUM) -o jsonpath="{.items[0].metadata.name}"); \
	kubectl exec $$DEV_POD_NAME -n ls$(NS_NUM) -- /bin/bash -c "$(CMD)";

deploy-cleanup: check-ls-num
	helm uninstall localstack --namespace ls$(NS_NUM);
	kubectl delete namespace ls$(NS_NUM);
