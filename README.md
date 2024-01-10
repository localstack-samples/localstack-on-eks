# 🌐 Overview

This blueprint has two solutions:

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

### Solution 1

This solution has the EKS cluster deployed on AWS.

#### Prerequisites for Solution-1 🧰

- An AWS Account
- [install Helm](https://helm.sh/docs/intro/install/)
- [install kubectl](https://kubernetes.io/docs/tasks/tools/)
- [install eksctl](https://eksctl.io/installation/)

#### Get credentials to your AWS account

#### Create EKS Cluster

This will create a new EKS cluster with a Fargate backend in your AWS Account, along with a new VPC.
```shell
export CLUSTER_NAME=lseksctl-cluster
export CLUSTER_REGION=us-west-2
eksctl create cluster --name $CLUSTER_NAME --region $CLUSTER_REGION --version 1.28 --fargate
```

#### Create a K8S namespace

```shell
kubectl create namespace eks-lstack1-ns
```

#### Create an EKS Fargate Profile

```shell
eksctl create fargateprofile \
    --cluster $CLUSTER_NAME \
    --name ls-fargate-profile \
    --namespace eks-lstack1-ns 
```

#### Set up cluster DNS

Apply patch to CoreDNS to leverage Localstack's DNS instead for all `localhost.localstack.cloud` requests.

```shell
kubectl apply -f manifests/coredns/eks.aws.yaml
kubectl rollout restart -n kube-system deployment/coredns
```

Get CoreDNS config by running the following:

```shell
kubectl get -n kube-system configmaps coredns -o yaml
```

Make Localstack's DNS discoverable by creating the following service:

```shell
kubectl apply -f manifests/coredns/ls-dns.yaml
```

### Solution 2

This solution has the EKS cluster deployed on your local machine, using the EKS anywhere plugin.

#### Prerequisites for Solution-2 🧰

- A laptop
- [install Helm](https://helm.sh/docs/intro/install/)
- [install kubectl](https://kubernetes.io/docs/tasks/tools/)
- [install eksctl](https://eksctl.io/installation/)
- [install eksanywhere plugin](https://anywhere.eks.amazonaws.com/docs/getting-started/install/)

#### Create EKS anywhere cluster

The following cluster creating takes about 5 minutes.

```shell
export CLUSTER_NAME=lseksctl-cluster
eksctl anywhere create cluster -f clusters/eks-anywhere/$CLUSTER_NAME.yaml -v 6
```

Export kubeconfig config:

```shell
export KUBECONFIG=${PWD}/${CLUSTER_NAME}/${CLUSTER_NAME}-eks-a-cluster.kubeconfig
```

#### Create a K8S namespace

```shell
kubectl create namespace eks-lstack1-ns
```

#### Set up cluster DNS

Apply patch to CoreDNS to leverage Localstack's DNS instead for all `localhost.localstack.cloud` requests.

```shell
kubectl apply -f manifests/coredns/eks.anywhere.yaml
kubectl rollout restart -n kube-system deployment/coredns
```

Get CoreDNS config by running the following:

```shell
kubectl get -n kube-system configmaps coredns -o yaml
```

Make Localstack's DNS discoverable by creating the following service:

```shell
kubectl apply -f manifests/coredns/ls-dns.yaml
```

### Deploy Apps

All following instructions are identical on both solutions (1 & 2).

#### Deploy sample application (optional)

https://docs.aws.amazon.com/eks/latest/userguide/sample-deployment.html

```shell
kubectl apply -f manifests/sample-app
```

#### Inspect deployed service

List the deployed services:

```shell
kubectl get all -n eks-lstack1-ns
```

View details of the deployed service:

```shell
kubectl -n eks-lstack1-ns describe service eks-sample-linux-service
```

Run a shell on a pod that you just gotten previously:

```shell
export RANDOM_POD_NAME=$(kubectl get pods -l "app=eks-sample-linux-app" -n eks-lstack1-ns -o jsonpath="{.items[0].metadata.name}")
kubectl exec -it $RANDOM_POD_NAME -n eks-lstack1-ns -- /bin/bash
```

Finally, from inside the pod, curl the endpoint by using the service name:

```shell
curl -i eks-sample-linux-service
```

#### Update helm config with LocalStack Pro

You can use this chart with LocalStack Pro by:

- Changing the image to localstack/localstack-pro.
- Providing your Auth Token as an environment variable.

Let's generate our `values.yaml` helm spec by substituting the image name and the auth token using the [charts/localstack/values.template.yaml](charts/localstack/values.template.yaml):

```shell
export LOCALSTACK_IMAGE_NAME="localstack/localstack-pro"
export LOCALSTACK_AUTH_TOKEN="<your auth token>"
envsubst < charts/localstack/values.template.yaml > charts/localstack/values.yaml
```

#### Deploy LocalStack

And you can use these values when installing the chart in your cluster:

```shell
helm repo add localstack-charts https://localstack.github.io/helm-charts
helm install localstack localstack-charts/localstack -f charts/localstack/values.yaml --namespace eks-lstack1-ns
```

**Warning: you temporarily need to use the helm chart provided on branch `nameserver-config`. `git clone -b nameserver-config https://github.com/localstack/helm-charts/tree/nameserver-config` in a different directory `$LOCALSTACK_CHARTS_DIR`. Following that, run `helm install localstack ./$LOCALSTACK_CHARTS_DIR/charts/localstack -f charts/localstack/values.yaml --namespace eks-lstack1-ns`**.

#### Get LocalStack container logs

Example

```shell
export LS_POD_NAME=$(kubectl get pods -l "app.kubernetes.io/name=localstack" -n eks-lstack1-ns -o jsonpath="{.items[0].metadata.name}")
kubectl logs $LS_POD_NAME -n eks-lstack1-ns
```

#### Install devpod GDC
```shell
kubectl apply -f manifests/devxpod/deployment.yaml
```

## Test Solution-1 / Solution-2

Now EKS is deployed with a unique namespace. LocalStack and the DevPod are both running.

After opening the shell, the command that follow are in the DevPod.

```shell
export DEV_POD_NAME=$(kubectl get pods -l "app=devxpod" -n eks-lstack1-ns -o jsonpath="{.items[0].metadata.name}")
kubectl exec -it $DEV_POD_NAME -n eks-lstack1-ns -- /bin/bash
```

Clone the repos we're testing. In an actual scenario, you might clone multiple repos, and/or restore LocalStack
CloudPod state before running tests.

```shell
git clone https://github.com/localstack-samples/lambda-ddb.git
```

Get into the repo dir.

```shell
cd lambda-ddb
```

Bootstrap the solution. This is a solution built with AWS CDK.

```shell
make integ-awscdk-bootstrap
```

Deploy the AWS CDK solution.

```shell
make integ-awscdk-deploy
```

Test the deploy AWS CDK solution.

```shell
make integ-awscdk-test
```

### Cleanup Solution-1

#### Uninstall LocalStack

Delete the localstack helm release, remove the namespace, and then delete the fargate profile.

```shell
helm uninstall localstack --namespace eks-lstack1-ns
kubectl delete namespace eks-lstack1-ns
eksctl delete fargateprofile \
    --cluster lseksctlCluster \
    --name ls-fargate-profile 
```

#### Delete the cluster

You have to wait a bit for the delete profile to clean up before doing this command.
This command will also take a couple minutes to cleanup the VPC that was created when this EKS cluster was created.

```shell
eksctl delete cluster --name $CLUSTER_NAME --region $CLUSTER_REGION
```

### Cleanup Solution-2

Delete the localstack helm release & remove the namespace.

```shell
helm uninstall localstack --namespace eks-lstack1-ns
kubectl delete namespace eks-lstack1-ns
```

And then finally, delete the cluster

```shell
eksctl anywhere delete cluster -f clusters/eks-anywhere/$CLUSTER_NAME.yaml
rm -r $CLUSTER_NAME
```
