#!/usr/bin/env bash

set -e

CHANGE_MINIKUBE_NONE_USER=true
K8S_VERSION=v1.9.4

# download and install kubectl, which is a requirement of minikube
curl -Lo kubectl https://storage.googleapis.com/kubernetes-release/release/${K8S_VERSION}/bin/linux/amd64/kubectl && chmod +x kubectl && sudo mv kubectl /usr/local/bin/

# download and install minikube
curl -Lo minikube https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64 && chmod +x minikube && sudo mv minikube /usr/local/bin/

# disable crash reports
minikube config set WantReportErrorPrompt false

# start minikube with none driver (it setups localkube on the host)
# see: https://github.com/kubernetes/minikube/issues/2704
sudo minikube start --bootstrapper=localkube --logtostderr --v=0 --vm-driver=none --kubernetes-version=${K8S_VERSION}

# update kubeconfig
minikube update-context

# wait for Kubernetes to be up and ready
JSONPATH='{range .items[*]}{@.metadata.name}:{range @.status.conditions[*]}{@.type}={@.status};{end}{end}'
until kubectl get nodes -o jsonpath="$JSONPATH" 2>&1 | grep -q "Ready=True"; do sleep 1; done