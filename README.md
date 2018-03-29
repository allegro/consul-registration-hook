# Consul Registration Hook

[![Build Status](https://travis-ci.org/allegro/consul-registration-hook.svg?branch=master)](https://travis-ci.org/allegro/consul-registration-hook)
[![Go Report Card](https://goreportcard.com/badge/github.com/allegro/consul-registration-hook)](https://goreportcard.com/report/github.com/allegro/consul-registration-hook)
[![GoDoc](https://godoc.org/github.com/allegro/consul-registration-hook?status.svg)](https://godoc.org/github.com/allegro/consul-registration-hook)

Hook that can be used for synchronous registration and deregistration in 
[Consul][1] discovery service on [Kubernetes][2] or [Mesos][3] cluster with 
Allegro [executor][4].

## Development

To develop the hook locally you need the following things to be installed on 
your machine:

* [Minikube][5]
* [Go][6]

When everything is installed and setup properly, you can build hook for the Linux 
operating system (as Minikube starts Kubernetes cluster on Linux virtual machine):

```
GOARCH=amd64 GOOS=linux go build -v ./cmd/consul-registration-hook
```

After successful build, you can start your local mini Kubernetes cluster with
project root mounted to the Kubernetes virtual machine:

```
minikube start --mount --mount-string .:/hooks
```

### Simple usecase, consul agent in separate container in the pod

Create a pod with Consul agent in development mode and hooks mounted:

```
kubectl create -f ./examples/service-for-dev.yaml
```

You can login to the container with hooks using the following command:

```
kubectl exec -it myservice-pod -- /bin/bash
```

### Consul ACL & DaemonSet usecase

Create consul secret:

```
kubectl create -f ./examples/secret-for-consul-agent.yaml
```

Create consul agent DaemonSet:

```
kubectl create -f ./examples/daemonset-with-acl-bootstrapping.yaml
```

Create service pod:

```
kubectl create -f ./examples/service-with-consul-lifecycle-hooks-and-acl-support.yaml
```

You can find the hook binary in `/hooks` folder on the container. All required
environment variables are set up so you can run a command without any additional
configuration.

[1]: https://www.consul.io/
[2]: https://kubernetes.io/
[3]: http://mesos.apache.org/
[4]: https://github.com/allegro/mesos-executor/
[5]: https://kubernetes.io/docs/getting-started-guides/minikube/
[6]: https://golang.org/doc/install
