#!/usr/bin/env bash

set -e

CHANGE_MINIKUBE_NONE_USER=true

# download and install kubectl, which is a requirement of minikube
curl -Lo kubectl https://storage.googleapis.com/kubernetes-release/release/v1.8.0/bin/linux/amd64/kubectl && chmod +x kubectl && sudo mv kubectl /usr/local/bin/

# download and install minikube
curl -Lo minikube https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64 && chmod +x minikube && sudo mv minikube /usr/local/bin/

# start minikube with none driver (it setups localkube on the host)
sudo minikube start --vm-driver=none --kubernetes-version=v1.8.0

# update kubeconfig
minikube update-context

# wait for Kubernetes to be up and ready
JSONPATH='{range .items[*]}{@.metadata.name}:{range @.status.conditions[*]}{@.type}={@.status};{end}{end}'
until kubectl get nodes -o jsonpath="$JSONPATH" 2>&1 | grep -q "Ready=True"; do sleep 1; done