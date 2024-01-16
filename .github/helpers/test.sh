#!/usr/bin/env bash

git clone https://github.com/localstack-samples/lambda-ddb.git
cd lambda-ddb

make integ-awscdk-bootstrap
make integ-awscdk-deploy
make integ-awscdk-test
