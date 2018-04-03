#!/usr/bin/env bash

set -e

sudo mkdir /hooks
sudo cp build/consul-registration-hook /hooks

echo "Starting myservice-pod..."
kubectl create -f examples/service-with-consul-lifecycle-hooks.yaml

until kubectl get pods | tee | grep Running
do
  echo "Waiting for myservice-pod to start successfully..."
  kubectl describe pod myservice-pod
  sleep 30
done

kubectl exec -it myservice-pod -- curl -v --fail http://localhost:8500/v1/catalog/service/myservice | grep myservice