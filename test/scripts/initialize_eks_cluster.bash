#!/bin/bash

# Copyright 2020 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

EKS_CLUSTER_NAME=$1
IMAGE_VERSION=$2

install_aws_cli() {
    curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"
    unzip awscliv2.zip > /dev/null 2>&1 
    ./aws/install
    aws --version
}

# install the latest aws-cli
install_aws_cli
# log the kubectl version
kubectl version --client=true

if [ -z "$AWS_REGION" ]; then
    AWS_REGION="us-west-2"
fi

AWS_ACCOUNT_ID=$(aws --region $AWS_REGION sts get-caller-identity --query Account --output text)

ECR_REGISTRY_NAME="driver"
ECR_CRD_REGISTRY_NAME="driver-crds"
ECR_REGISTRY_URL="$AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com"
ECR_REGISTRY_URL_WITH_NAME="$ECR_REGISTRY_URL/$ECR_REGISTRY_NAME"
ECR_CRD_REGISTRY_URL_WITH_NAME="$ECR_REGISTRY_URL/$ECR_CRD_REGISTRY_NAME"

IMAGE_TAG=$ECR_REGISTRY_URL_WITH_NAME:$IMAGE_VERSION
CRD_IMAGE_TAG=$ECR_CRD_REGISTRY_URL_WITH_NAME:$IMAGE_VERSION
NAMESPACE="kube-system"
AWS_SERVICE_ACCOUNT_NAME="basic-test-mount-sa"

if [ -z "$RELEASE" ]; then
    aws --region $AWS_REGION ecr get-login-password | docker login --username AWS --password-stdin $ECR_REGISTRY_URL
    REGISTRY=$ECR_REGISTRY_URL make container
    docker push $IMAGE_TAG
    docker push $CRD_IMAGE_TAG
fi

eksctl create cluster --name $EKS_CLUSTER_NAME --node-type m5.large --region $AWS_REGION
# update kubeconfig for https://github.com/aws/aws-cli/issues/6920
aws eks update-kubeconfig --name $EKS_CLUSTER_NAME --region $AWS_REGION
eksctl utils associate-iam-oidc-provider --name $EKS_CLUSTER_NAME --approve --region $AWS_REGION
eksctl create iamserviceaccount \
     --name $AWS_SERVICE_ACCOUNT_NAME \
     --namespace $NAMESPACE \
     --cluster $EKS_CLUSTER_NAME \
     --attach-policy-arn arn:aws:iam::aws:policy/AmazonSSMReadOnlyAccess \
     --attach-policy-arn arn:aws:iam::aws:policy/SecretsManagerReadWrite \
     --override-existing-serviceaccounts \
     --approve \
     --region $AWS_REGION

# on a release test the caller will perform the driver installation
if [ -z "$RELEASE" ]; then
    REGISTRY=$ECR_REGISTRY_URL make e2e-helm-deploy
fi
