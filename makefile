SHELL := /bin/bash

-include .env
-include .env-local

.PHONY: \
    aws-create-cluster \
    aws-delete-cluster \
    local-create-cluster \
    local-delete-cluster \
	create-namespace \
	deploy-localstack \
	deploy-cleanup \
	exec-devpod-interactive

######################
# Helper targets     #
######################

check-auth-token:
ifndef LOCALSTACK_AUTH_TOKEN
	$(error LOCALSTACK_AUTH_TOKEN is not set)
endif

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
	eksctl utils write-kubeconfig --cluster $(CLUSTER_NAME) --region $(CLUSTER_REGION);

aws-delete-cluster:
	eksctl delete cluster --name $(CLUSTER_NAME) --region $(CLUSTER_REGION);
	rm -r $(CLUSTER_NAME) eksa-cli-logs;

aws-create-fargate-profile: check-ls-num create-namespace
	eksctl create fargateprofile \
		--cluster $(CLUSTER_NAME) \
		--region $(CLUSTER_REGION) \
		--name ls-fargate-profile$(NS_NUM) \
		--namespace ls$(NS_NUM);

aws-delete-fargate-profile: check-ls-num delete-namespace
	eksctl delete fargateprofile \
		--cluster $(CLUSTER_NAME) \
		--region $(CLUSTER_REGION) \
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

create-namespace: check-ls-num check-auth-token
	kubectl create namespace ls$(NS_NUM) --dry-run=client -o yaml | kubectl apply -f -;
	kubectl create secret generic localstack-auth-token --from-literal=LOCALSTACK_AUTH_TOKEN=$(LOCALSTACK_AUTH_TOKEN) -n ls$(NS_NUM) --dry-run=client -o yaml | kubectl apply -f -;

delete-namespace: check-ls-num
	kubectl delete namespace ls$(NS_NUM) --ignore-not-found=true;

deploy-localstack: check-ls-num
	envsubst < manifests/gdc-template.yaml > manifests/gdc.yaml;
	kubectl apply -f manifests/gdc.yaml;

	envsubst < manifests/localstack-template.yaml > manifests/localstack.yaml;
	kubectl apply -f manifests/localstack.yaml;

exec-devpod-interactive: check-ls-num
	DEV_POD_NAME=$(shell kubectl get pods -l app=devxpod -n ls$(NS_NUM) -o jsonpath="{.items[0].metadata.name}"); \
	kubectl exec -it $$DEV_POD_NAME -n ls$(NS_NUM) -- /bin/bash;

exec-devpod-noninteractive: check-ls-num check-cmd
	DEV_POD_NAME=$(shell kubectl get pods -l app=devxpod -n ls$(NS_NUM) -o jsonpath="{.items[0].metadata.name}"); \
	kubectl exec $$DEV_POD_NAME -n ls$(NS_NUM) -- /bin/bash -c "$(CMD)";

deploy-cleanup: check-ls-num
	kubectl delete namespace ls$(NS_NUM);
