package k8s

import (
	"context"
	"fmt"
	"os"

	"github.com/ericchiang/k8s"
	corev1 "github.com/ericchiang/k8s/apis/core/v1"

	"github.com/allegro/consul-registration-hook/consul"
)

const (
	consulLabelKey     = "consul"
	podNamespaceEnvVar = "KUBERNETES_POD_NAMESPACE"
	podNameEnvVar      = "KUBERNETES_POD_NAME"
)

// Client is an interface for client to Kubernetes API.
type Client interface {
	// GetPod returns current pod data.
	GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error)
}

type defaultClient struct {
}

func (c *defaultClient) GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error) {
	client, err := k8s.NewInClusterClient()
	if err != nil {
		return nil, fmt.Errorf("unable to create k8s client: %s", err)
	}

	pod := &corev1.Pod{}
	if err := client.Get(ctx, namespace, name, pod); err != nil {
		return nil, fmt.Errorf("unable to get pod data from API: %s", err)
	}

	return pod, nil
}

// ServiceProvider is responsible for providing services that should be registered
// in Consul discovery service.
type ServiceProvider struct {
	Client Client
}

// Get returns slice of services that are configured to be registered in Consul
// discovery service.
func (p *ServiceProvider) Get(ctx context.Context) ([]consul.ServiceInstance, error) {
	client := p.client()

	podNamespace := os.Getenv(podNamespaceEnvVar)
	podName := os.Getenv(podNameEnvVar)

	pod, err := client.GetPod(ctx, podNamespace, podName)
	if err != nil {
		return nil, fmt.Errorf("unable to get pod data from API: %s", err)
	}

	serviceName, ok := pod.Metadata.Labels[consulLabelKey]
	if !ok {
		return nil, nil
	}

	// TODO(medzin): Allow to specify which containers and ports will be registered
	container := pod.Spec.Containers[0]
	port := int(*container.Ports[0].HostPort)

	service := consul.ServiceInstance{
		ID:    fmt.Sprintf("%s_%s_%s_%d", podName, podName, *container.Name, port),
		Name:  serviceName,
		Host:  os.Getenv("KUBERNETES_SERVICE_HOST"),
		Port:  int(*container.Ports[0].HostPort),
		Check: ConvertToConsulCheck(container.LivenessProbe),
	}

	return []consul.ServiceInstance{service}, nil
}

func (p *ServiceProvider) client() Client {
	if p.Client != nil {
		return p.Client
	}
	return &defaultClient{}
}
