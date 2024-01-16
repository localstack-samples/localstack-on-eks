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
Multiple namespaces isolate testing of different solutions.
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

### Solution-1

This solution has the EKS cluster deployed on AWS.

#### Prerequisites for Solution-1 🧰

- An AWS Account
- [install Helm](https://helm.sh/docs/intro/install/)
- [install kubectl](https://kubernetes.io/docs/tasks/tools/)
- [install eksctl](https://eksctl.io/installation/)

#### Get credentials to your AWS account

#### Create a file named `.env-local`
Put these contents in it
```shell
export LOCALSTACK_AUTH_TOKEN=<your LocalStack auth token>
```

#### Solution-1 Steps

**Setup Cluster and CoreDNS**
This blueprint builds namespaces in the format of `ls<NS_NUM>`. So, we're going
to choose a Namespace number for the following targets. 
```shell
make aws-setup-cluster NS_NUM=0
```

**Create Namespaces and Fargate Profiles**
```shell
make aws-setup-ns NS_NUM=0
```

**Deploy LocalStack and Dev Pod Namespace ls0**
```shell
make  aws-deploy-ls NS_NUM=0
```

**Open Shell to DevPod**
```shell
make aws-ssh-devpod NS_NUM=0
```

**Clone solution repo and test**
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

Restart LocalStack inside the running Pod. Now you can deploy again and retest.

```shell
make reset-ls
```

**Cleanup EKS Cluster**
make aws-cleanup-ns NS_NUM=<your namespace number>
```shell
make aws-cleanup-ns NS_NUM=0
make aws-cleanup-cluster
```


### Solution-2

This solution has the EKS cluster deployed on your local machine, using the EKS anywhere plugin.

#### Prerequisites for Solution-2 🧰

- A laptop
- [install Helm](https://helm.sh/docs/intro/install/)
- [install kubectl](https://kubernetes.io/docs/tasks/tools/)
- [install eksctl](https://eksctl.io/installation/)
- [install eksanywhere plugin](https://anywhere.eks.amazonaws.com/docs/getting-started/install/)

#### Create EKS anywhere cluster

Creating the cluster takes a couple minutes. Select a K8S Namespace number and supply it as an argument.

```shell
make eksany-create-cluster NS_NUM=0
```

Create Namespace and setup Coredns
```shell
make eksany-setup-ns NS_NUM=0
```

#### Deploy LocalStack and DevPod

```shell
make eksany-deploy-ls NS_NUM=0
```

#### Run Test
Get a shell on the DevPod
```shell
make eksany-ssh-devpod NS_NUM=0
```

**Clone solution repo and test**
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

#### Setup Second Namespace
To setup a second namespace and test in it, simply do this. We'll use namespace number 2.
The cluster already exists so we'll just add another namespace to it,
deploy LocalStack and the DevPod, login to the DevPod, and test a repo.
```shell
make eksany-setup-ns NS_NUM=2
make eksany-deploy-ls NS_NUM=2
make eksany-ssh-devpod NS_NUM=2
git clone https://github.com/localstack-samples/lambda-ddb.git
cd lambda-ddb
make integ-awscdk-bootstrap
make integ-awscdk-deploy
make integ-awscdk-test
```
**Cleanup Namespace 2**
```shell
make eksany-cleanup-ns NS_NUM=2
```

### Misc Commands
**Edit Coredns config directly**
```shell
kubectl edit -n kube-system configmaps coredns
```

**List resources in namespace ls0**
```shell
kubectl get all -n ls0
```

**Get LocalStack container logs**
```shell
make eksany-lslogs NS_NUM=0
```
