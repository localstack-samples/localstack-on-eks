# 🌐 Overview
This blueprint has two solutions
1. Deploy LocalStack to AWS EKS with Fargate. 
2. Deploy LocalStack on an engineer's laptop on EKS Anywhere with Docker.

**Solution-1** provides a hybrid integration environment where teams can run component/integration/system tests.
The solution is managed in AWS to allow for easy management of the entire platform across multiple AWS accounts.

**Solution-2** is identical to Solution-1 but it runs on engineers laptops with EKS Anywhere. 

The two solutions having nearly identical tooling allows enterprise teams to create a manageable
solution testing platform.

### LocalStack on AWS EKS Fargate
![LSonEKS](./docs/design-ls-on-aws-eks.drawio.png "LSonEKS")

### LocalStack on Engineer's Laptop with EKS Anywhere
![LSonEKSAny](./docs/design-ls-on-eksany.drawio.png "LSonEKSAny")

### 🔑 Key Components

- **LocalStack on K8S**
    LocalStack provides AWS Service emulation to create aan amazing DevX with powerful solution testing. 
- **Dev Container**
    Provides standard tooling to build, deploy, and test solutions. 
- **Amazon Elastic Kubernetes Service**
    K8S common platform for DevSecOps tooling to support unit, component, and integration testing.

## Getting Started 🏁

This guide assumes that you have cloned this repository and are in the project root directory. The following steps will
guide you through the process of building, deploying, and test Solution-1 (Solution-2 link).
Solution-1 is not free. It will cost money to run EKS in AWS. Make sure to destroy your resources in the cleanup
section to control your costs.

### Prerequisites for Solution-1 🧰
- An AWS Account
- [install Helm](https://helm.sh/docs/intro/install/)
- [install kubectl](https://kubernetes.io/docs/tasks/tools/)
- [install eksctl](https://eksctl.io/installation/)

### Get credentials to your AWS account

### Create EKS Cluster
This will create a new EKS cluster with a Fargate backend in your AWS Account, along with a new VPC.
```shell
eksctl create cluster --name lseksctlCluster --region us-west-2 --version 1.28 --fargate
```

### Update Core DNS
```shell
kubectl apply -f coredns.yaml
kubectl rollout restart -n kube-system deployment/coredns
```
To see the core dns you can run this
```shell
kubectl get -n kube-system configmaps coredns -o yaml
```

### Create a K8S namespace
```shell
kubectl create namespace eks-lstack1-ns
```

### Create an EKS Fargate Profile
```shell
eksctl create fargateprofile \
    --cluster lseksctlCluster \
    --name ls-fargate-profile \
    --namespace eks-lstack1-ns 
```

### Deploy LocalStack DNS service
```shell
kubectl apply -f ls-dns.yaml
```

### Deploy sample application (optional)
https://docs.aws.amazon.com/eks/latest/userguide/sample-deployment.html
```shell
kubectl apply -f sample-app.yaml
```

### Create sample K8S service
```shell
kubectl apply -f sample-service.yaml
```

### List all the services
```shell
kubectl get all -n eks-lstack1-ns
```

### View details of the deployed service
```shell
kubectl -n eks-lstack1-ns describe service eks-sample-linux-service
```

### Run a shell on a pod that you described in List all the services step
Replacing pod name with name of your pod
```shell
kubectl exec -it eks-sample-linux-deployment-5b568bf897-cv5zx -n eks-lstack1-ns -- /bin/bash
```
#### Curl the endpoint
```shell
curl eks-sample-linux-service
```

### Update helm config with LocalStack pro
You can use this chart with LocalStack Pro by:
- Changing the image to localstack/localstack-pro.
- Providing your Auth Token as an environment variable.
You can set these values in a YAML file (in this example pro-values.yaml):
```yaml
image:
  repository: localstack/localstack-pro
  tag: "latest"

service:
  clusterIP: "10.100.0.42"

extraEnvVars:
  - name: LOCALSTACK_AUTH_TOKEN
    value: "<your LocalStack auth Token>"
  - name: GATEWAY_LISTEN
    value: "0.0.0.0:4566"
  - name: DNS_RESOLVE_IP
    value: "10.100.0.42"

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

## Test Solution-1
Now EKS is deployed with a unique namespace. LocalStack and the DevPod are both running.
### Open a shell in the DevPod
After opening the shell, the command that follow are in the DevPod.
```shell
kubectl exec -it <podname> -n eks-lstack1-ns -- /bin/bash
```
**Clone Repo(s)**
Clone the repos we're testing. In an actual scenario, you might clone multiple repos, and/or restore LocalStack
CloudPod state before running tests.
```shell
git clone https://github.com/localstack-samples/lambda-ddb.git
```
Get into the repo dir.
```shell
cd lambda-ddb
```
**Bootstrap the solution** - This is a solution built with AWS CDK.
```shell
make integ-awscdk-bootstrap
```
**Deploy the solution**
```shell
make integ-awscdk-deploy
```
**Test the solution**
```shell
make integ-awscdk-test
```


## Cleanup Solution-1

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