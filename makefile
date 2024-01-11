SHELL := /bin/bash

-include .env
-include .env-local

.PHONY: aws-setup-cluster aws-deploy-all-ns aws-deploy-ls aws-ssh-devpod0 aws-ssh-devpod1 aws-cleanup-ns0 aws-cleanup-ns1 aws-cleanup-cluster aws-setup-ns0 aws-setup-ns1 aws-setup-nss

aws-setup-cluster:
	eksctl create cluster --name $(CLUSTER_NAME) --region $(CLUSTER_REGION) --version 1.28 --fargate;
	kubectl apply -f manifests/coredns/eks.aws.yaml;
	kubectl rollout restart -n kube-system deployment/coredns;
	kubectl apply -f manifests/coredns/ls-dns.yaml;

aws-setup-ns0:
	kubectl create namespace $(LS_K8S_NS0);
	eksctl create fargateprofile \
    --cluster $(CLUSTER_NAME) \
    --name ls-fargate-profile0 \
    --namespace $(LS_K8S_NS0);

aws-setup-ns1:
	kubectl create namespace $(LS_K8S_NS1);
	eksctl create fargateprofile \
    --cluster $(CLUSTER_NAME) \
    --name ls-fargate-profile1 \
    --namespace $(LS_K8S_NS1);

aws-setup-nss: aws-setup-ns0 aws-setup-ns1

aws-deploy-setup:
	envsubst < charts/localstack/values.template.yaml > charts/localstack/values.yaml;
	helm repo add localstack-charts https://localstack.github.io/helm-charts;

aws-deploy-ls0: aws-deploy-setup
	helm install localstack localstack-charts/localstack -f charts/localstack/values.yaml --namespace $(LS_K8S_NS0);
	kubectl apply -f manifests/devxpod/deployment-ns0.yaml;

aws-deploy-ls1: aws-deploy-setup
	helm install localstack localstack-charts/localstack -f charts/localstack/values.yaml --namespace $(LS_K8S_NS1);
	kubectl apply -f manifests/devxpod/deployment-ns1.yaml;

aws-deploy-all-ns: aws-deploy-ls0 aws-deploy-ls1

# Set target specific variable DEV_POD_NAME to be used in that target
aws-ssh-devpod0: DEV_POD_NAME=$(shell kubectl get pods -l app=devxpod -n $(LS_K8S_NS0) -o jsonpath="{.items[0].metadata.name}")
aws-ssh-devpod0:
	kubectl exec -it $(DEV_POD_NAME) -n $(LS_K8S_NS0) -- /bin/bash;

aws-ssh-devpod1: DEV_POD_NAME=$(shell kubectl get pods -l app=devxpod -n $(LS_K8S_NS1) -o jsonpath="{.items[0].metadata.name}")
aws-ssh-devpod1:
	kubectl exec -it $(DEV_POD_NAME) -n $(LS_K8S_NS1) -- /bin/bash;

aws-cleanup-ns0:
	helm uninstall localstack --namespace $(LS_K8S_NS0)
	kubectl delete namespace $(LS_K8S_NS0)
	eksctl delete fargateprofile \
		--cluster $(CLUSTER_NAME) \
		--name ls-fargate-profile0

aws-cleanup-ns1:
	helm uninstall localstack --namespace $(LS_K8S_NS1)
	kubectl delete namespace $(LS_K8S_NS1)
	eksctl delete fargateprofile \
		--cluster $(CLUSTER_NAME) \
		--name ls-fargate-profile1

aws-cleanup-nss: aws-cleanup-ns0 aws-cleanup-ns1

aws-cleanup-cluster:
	eksctl delete cluster --name $(CLUSTER_NAME) --region $(CLUSTER_REGION)
