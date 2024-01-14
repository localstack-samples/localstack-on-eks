SHELL := /bin/bash

-include .env
-include .env-local

.PHONY: gen-coredns aws-setup-cluster aws-deploy-all-ns aws-deploy-ls aws-ssh-devpod0 aws-ssh-devpod1 aws-cleanup-ns0 aws-cleanup-ns1 aws-cleanup-cluster aws-setup-ns0 aws-setup-ns1 aws-setup-nss

aws-setup-cluster:
	eksctl create cluster --name $(CLUSTER_NAME) --region $(CLUSTER_REGION) --version 1.28 --fargate;

gen-coredns:
	kubectl get -n kube-system configmaps coredns -o yaml | \
	yq  '.data.Corefile = (.data.Corefile + "\nlocalstack$(NS_NUM):53 {\n    errors\n    cache 5\n    forward . 10.100.$(NS_NUM).53\n}")' | \
	yq 'del(.metadata.annotations, .metadata.resourceVersion, .metadata.uid, .metadata.creationTimestamp)' \
	>> coredns-tmp.yaml

aws-setup-ns: gen-coredns
	kubectl create namespace ls$(NS_NUM);
	eksctl create fargateprofile \
    --cluster $(CLUSTER_NAME) \
    --name ls-fargate-profile$(NS_NUM) \
    --namespace ls$(NS_NUM);
	kubectl apply -f coredns-tmp.yaml;
	kubectl rollout restart -n kube-system deployment/coredns;
	envsubst < manifests/coredns/ls-dns-template.yaml > manifests/coredns/ls-dns-gen.yaml
	kubectl apply -f manifests/coredns/ls-dns-gen.yaml;

aws-deploy-setup: export NODE_PORT=$(shell expr 31566 + ${NS_NUM})
aws-deploy-setup:
	envsubst < charts/localstack/values.template.yaml > charts/localstack/values.yaml;
	envsubst < manifests/devxpod/deployment-template.yaml > manifests/devxpod/deployment-gen.yaml;
	helm repo add localstack-charts https://localstack.github.io/helm-charts;

aws-deploy-ls: aws-deploy-setup
	helm install localstack localstack-charts/localstack -f charts/localstack/values.yaml --namespace ls$(NS_NUM);
	kubectl apply -f manifests/devxpod/deployment-gen.yaml;

# Set target specific variable DEV_POD_NAME to be used in that target
aws-ssh-devpod: DEV_POD_NAME=$(shell kubectl get pods -l app=devxpod -n ls$(NS_NUM) -o jsonpath="{.items[0].metadata.name}")
aws-ssh-devpod:
	kubectl exec -it $(DEV_POD_NAME) -n ls$(NS_NUM) -- /bin/bash;


aws-cleanup:
	helm uninstall localstack --namespace ls$(NS_NUM)
	kubectl delete namespace ls$(NS_NUM)
	eksctl delete fargateprofile \
		--cluster $(CLUSTER_NAME) \
		--name ls-fargate-profile$(NS_NUM)

aws-cleanup-cluster:
	eksctl delete cluster --name $(CLUSTER_NAME) --region $(CLUSTER_REGION)
