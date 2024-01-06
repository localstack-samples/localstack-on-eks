# localstack-on-eks
DevOps blueprint to run LocalStack on EKS in AWS

# Setup
Install 
- helm
- kubectl
- eksctl

# Steps
## Get credentials to your AWS account

## Create EKS Cluster 
```shell
eksctl create cluster --name lseksctlCluster --region us-west-2 --version 1.28 --fargate
```

## Create a K8S namespace
https://docs.aws.amazon.com/eks/latest/userguide/sample-deployment.html
```shell
kubectl create namespace eks-lstack1-ns
```

## Create an EKS Fargate Profile
```shell
eksctl create fargateprofile \
    --cluster lseksctlCluster \
    --name ls-fargate-profile \
    --namespace eks-lstack1-ns 
```

## Deploy sample application
```shell
kubectl apply -f sample-app.yaml
```

## Create sample K8S service
```shell
kubectl apply -f sample-service.yaml
```

## List all the services
```shell
kubectl get all -n eks-lstack1-ns
```

## View details of the deployed service
```shell
kubectl -n eks-lstack1-ns describe service eks-sample-linux-service
```

## Run a shell on a pod that you described in List all the services step
Replacing pod name with name of your pod
```shell
kubectl exec -it eks-sample-linux-deployment-5b568bf897-cv5zx -n eks-lstack1-ns -- /bin/bash
```
### Curl the endpoint
```shell
curl eks-sample-linux-service
```

## Update helm config with LocalStack pro
You can use this chart with LocalStack Pro by:
- Changing the image to localstack/localstack-pro.
- Providing your Auth Token as an environment variable.
You can set these values in a YAML file (in this example pro-values.yaml):
```yaml
image:
  repository: localstack/localstack-pro
  tag: "latest"

nameOverride: "localstack"
fullnameOverride: "localstack"

extraEnvVars:
  - name: LOCALSTACK_AUTH_TOKEN
    value: "<your LocalStack auth token>"

# enable debugging
debug: true

lambda:
  # The lambda runtime executor.
  # Depending on the value, LocalStack will execute lambdas either in docker containers or in kubernetes pods
  # The value "kubernetes" depends on the service account and pod role being activated
  executor: "kubernetes"
  environment_timeout: 400

  security_context:
    runAsUser: 1000
    fsGroup: 1000

resources:
  requests:
    cpu: 1000m
    memory: 4Gi

readinessProbe:
  initialDelaySeconds: 60

livenessProbe:
  initialDelaySeconds: 60
```

### Deploy LocalStack
And you can use these values when installing the chart in your cluster:
```shell
helm repo add localstack-charts https://localstack.github.io/helm-charts
helm install localstack localstack-charts/localstack -f eks-values.yaml --namespace eks-lstack1-ns
```

### Get LocalStack container log
Example
kubectl logs <podname> -n eks-lstack1-ns
```shell
kubectl logs localstack-854d8fdc8-q6lr2 -n eks-lstack1-ns
```

### Install devpod GDC
```shell
kubectl apply -f devxpod.yaml
```

### Uninstall LocalStack
```shell
helm uninstall localstack --namespace eks-lstack1-ns
```

## Delete the namespace
```shell
kubectl delete namespace eks-lstack1-ns
```

## Delete fargate profile
```shell
eksctl delete fargateprofile \
    --cluster lseksctlCluster \
    --name ls-fargate-profile 
```

## Delete the cluster
You have to wait a bit for the delete profile to clean up before doing this command.
This command will also take a couple minutes to cleanup the VPC that was created when this EKS cluster was created.
```shell
eksctl delete cluster --name lseksctlCluster --region us-west-2
```