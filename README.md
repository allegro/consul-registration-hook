# Consul Registration Hook

[![Build Status](https://github.com/allegro/consul-registration-hook/actions/workflows/golangci.yaml/badge.svg)](https://github.com/allegro/consul-registration-hook/actions/workflows/golangci.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/allegro/consul-registration-hook)](https://goreportcard.com/report/github.com/allegro/consul-registration-hook)
[![Codecov](https://codecov.io/gh/allegro/consul-registration-hook/branch/master/graph/badge.svg)](https://codecov.io/gh/allegro/consul-registration-hook)
[![GoDoc](https://godoc.org/github.com/allegro/consul-registration-hook?status.svg)](https://godoc.org/github.com/allegro/consul-registration-hook)

Hook that can be used for synchronous registration and deregistration in
[Consul][1] discovery service on [Kubernetes][2] or [Mesos][3] cluster with
Allegro [executor][4].

## Why hook uses synchronous communication

aSynchronous communication with Consul allows to achieve a gracefull shutdown of
old application version during the deployment. New instances are considered
running and healthy when they are registered succesfully in discovery service.
Old instances are first deregistered and then killed with configurable delay,
which allows to propagate deregistration across whole Consul cluster and its
clients.

Synchronous communication has one drawback - deregistration from Consul may never
take place. This situation is mitigated by forcing to use `DeregisterCriticalServiceAfter`
field in Consul checks, which deregisters automatically instances that are
unhealthy for too long. The time after which unhealthy instances are removed can
be long enough that some other application will start up on the same address and
start responding to Consul checks - this is mitigated by using service ID
composed from IP and port of the instance that should be registered. This results
in overwriting the old obsolete instance with a new one, accelerating the
cleaning of the Consul service catalog.

## Usage

### Kubernetes

On Kubernetes the hook is fired by using [Container Lifecycle Hooks][7]:

```yaml
# container
lifecycle:
  postStart:
    exec:
      command: ["/bin/sh", "-c", "/hooks/consul-registration-hook register k8s"]
  preStop:
    exec:
      command: ["/bin/sh", "-c", "/hooks/consul-registration-hook deregister k8s"]
```

Hook requires additional configuration passed by environmental variables. Because
the pod name and namespace is not passed by default to the container they have
to be passed manually:

```yaml
# container
env:
  - name: KUBERNETES_POD_NAME
    valueFrom:
      fieldRef:
        fieldPath: metadata.name
  - name: KUBERNETES_POD_NAMESPACE
    valueFrom:
      fieldRef:
        fieldPath: metadata.namespace
```

Optionally, if Consul agent requires token for authentication it can be passed
by using [Secrets][8]:

```yaml
containers:
# ... other configuration ...
    volumeMounts:
      - name: consul-acl
        mountPath: /consul-acl
    lifecycle:
    postStart:
      exec:
        command: ["/bin/sh", "-c", "/hooks/consul-registration-hook --consul-acl-file /consul-acl/token register k8s"]
    preStop:
      exec:
        command: ["/bin/sh", "-c", "/hooks/consul-registration-hook --consul-acl-file /consul-acl/token deregister k8s"]
# ... other configuration ...
volumes:
  - name: consul-acl
    secret:
      secretName: consul-acl
      items:
      - key: agent-token
        path: token
        mode: 511
```

#### Production

It is recommended to have a local copy of the hook on the production environment.
For example on Google Cloud Platform you can have a copy of the hook in dedicated
Cloud Storage bucket. Then you can authorize Compute Engine service account to
have read only access to the bucket. After everything is prepared you can use
[Init Container][9] to download hook and expose it on shared volume to the main
container:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: pod-with-consul-hook
  labels:
    consul: service-name
spec:
  initContainers:
  - name: hook-init-container
    image: google/cloud-sdk:alpine
    imagePullPolicy: Always
    command: ["/bin/sh"]
    args: ["-c", "gsutil cat ${GS_URL} | tar -C /hooks -zxvf -"]
    env:
    - name: GS_URL
        valueFrom:
          configMapKeyRef:
            name: consul-registration-hook
            key: GS_URL
    volumeMounts:
    - name: hooks
      mountPath: /hooks
  containers:
  - name: service-with-consul-hook-container
    image: python:2
    command: ["python", "-m", "SimpleHTTPServer", "8080"]
    env:
    - name: KUBERNETES_POD_NAME
      valueFrom:
        fieldRef:
          fieldPath: metadata.name
    - name: KUBERNETES_POD_NAMESPACE
      valueFrom:
        fieldRef:
          fieldPath: metadata.namespace
    - name: HOST_IP
      valueFrom:
        fieldRef:
          fieldPath: status.hostIP
    - name: CONSUL_HTTP_ADDR
      value: "$(HOST_IP):8500"
    ports:
    - containerPort: 8080
    volumeMounts:
    - name: hooks
      mountPath: /hooks
    lifecycle:
      postStart:
        exec:
          command: ["/bin/sh", "-c", "/hooks/consul-registration-hook register k8s"]
      preStop:
        exec:
          command: ["/bin/sh", "-c", "/hooks/consul-registration-hook deregister k8s"]
  volumes:
  - name: hooks
    emptyDir: {}
```

### Mesos

Registration based on data provided from Mesos API is supported only partially.
Because Mesos API do not provide health check definions we are unable to sync
them with Consul agent.

## Development

### Kubernetes integration

To develop the hook locally you need the following things to be installed on
your machine:

* [Minikube][5]
* [Go][6]

When everything is installed and setup properly, you can build hook for the Linux
operating system (as Minikube starts Kubernetes cluster on Linux virtual machine):

```bash
make build-linux
```

After successful build, you can start your local mini Kubernetes cluster with
project root mounted to the Kubernetes virtual machine:

```bash
minikube start --mount --mount-string .:/hooks
```

#### Simple usecase, consul agent in separate container in the pod

Create a pod with Consul agent in development mode and hooks mounted:

```bash
kubectl create -f ./examples/service-for-dev.yaml
```

You can login to the container with hooks using the following command:

```bash
kubectl exec -it myservice-pod -- /bin/bash
```

#### Consul ACL & DaemonSet usecase

Create consul secret:

```bash
kubectl create -f ./examples/secret-for-consul-agent.yaml
```

Create consul agent DaemonSet:

```bash
kubectl create -f ./examples/daemonset-with-acl-bootstrapping.yaml
```

Create service pod:

```bash
kubectl create -f ./examples/service-with-consul-lifecycle-hooks-and-acl-support.yaml
```

You can find the hook binary in `/hooks` folder on the container. All required
environment variables are set up so you can run a command without any additional
configuration.

### Mesos integration

To develop the hook locally you need the following things to be installed on
your machine:

* [Docker CE][10]
* [Go][6]

When everything is installed and setup properly, you can build hook for the Linux
operating system (we will use dockerized Mesos cluster for development):

```bash
make build-linux
```

After successful build, you can start your local Mesos + Marathon cluster:

```bash
docker-compose up
```

Hook binary is available on Mesos slave container in `/opt/consul-registration-hook/`
folder, and can be used directly when deploying apps using Marathon (localhost:8080).

[1]: https://www.consul.io/
[2]: https://kubernetes.io/
[3]: http://mesos.apache.org/
[4]: https://github.com/allegro/mesos-executor/
[5]: https://kubernetes.io/docs/getting-started-guides/minikube/
[6]: https://golang.org/doc/install
[7]: https://kubernetes.io/docs/concepts/containers/container-lifecycle-hooks/
[8]: https://kubernetes.io/docs/concepts/configuration/secret/
[9]: https://kubernetes.io/docs/concepts/workloads/pods/init-containers/
[10]: https://www.docker.com/get-docker
