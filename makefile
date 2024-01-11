SHELL := /bin/bash

-include .env
-include .env-local

.PHONY: aws-setup-cluster aws-deploy-ls aws-ssh-devpod

aws-setup-cluster:
	eksctl create cluster --name $(CLUSTER_NAME) --region $(CLUSTER_REGION) --version 1.28 --fargate;
	kubectl create namespace $(LS_K8S_NAMESPACE);
	eksctl create fargateprofile \
    --cluster $(CLUSTER_NAME) \
    --name ls-fargate-profile \
    --namespace $(LS_K8S_NAMESPACE);
	kubectl apply -f manifests/coredns/eks.aws.yaml;
	kubectl rollout restart -n kube-system deployment/coredns;
	kubectl apply -f manifests/coredns/ls-dns.yaml;

aws-deploy-ls:
	envsubst < charts/localstack/values.template.yaml > charts/localstack/values.yaml;
	helm repo add localstack-charts https://localstack.github.io/helm-charts;
	helm install localstack localstack-charts/localstack -f charts/localstack/values.yaml --namespace $(LS_K8S_NAMESPACE);
	kubectl apply -f manifests/devxpod/deployment.yaml;

# Set target specific variable DEV_POD_NAME to be used in that target
aws-ssh-devpod: DEV_POD_NAME=$(shell kubectl get pods -l app=devxpod -n $(LS_K8S_NAMESPACE) -o jsonpath="{.items[0].metadata.name}")
aws-ssh-devpod:
	kubectl exec -it $(DEV_POD_NAME) -n $(LS_K8S_NAMESPACE) -- /bin/bash;
