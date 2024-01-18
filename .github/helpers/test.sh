#!/usr/bin/env bash

git clone https://github.com/localstack-samples/lambda-ddb.git
cd lambda-ddb

# This is a workaround. The CDK deploy command is interactive and requires user input.
# Make sure the cdk deploy command works in non-interactive mode.
FIND_TEXT='$(CDK_CMD) deploy $(TFSTACK_NAME) --outputs-file stack-outputs-$(STACK_SUFFIX).json'
REPLACE_TEXT='$(CDK_CMD) deploy $(TFSTACK_NAME) --outputs-file stack-outputs-$(STACK_SUFFIX).json --require-approval=never'

FILE=$(grep -rl "$FIND_TEXT" $PWD)
if [ -z "$FILE" ]; then
    echo "The text was not found in any file."
    exit 1
else
    sed -i "s|$FIND_TEXT|$REPLACE_TEXT|g" $FILE
fi

make integ-awscdk-bootstrap
make integ-awscdk-deploy
make integ-awscdk-test
