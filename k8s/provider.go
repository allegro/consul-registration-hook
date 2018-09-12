package k8s

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/ericchiang/k8s"
	corev1 "github.com/ericchiang/k8s/apis/core/v1"

	"github.com/allegro/consul-registration-hook/consul"
)

const (
	consulLabelKey                  = "consul"
	podNamespaceEnvVar              = "KUBERNETES_POD_NAMESPACE"
	podNameEnvVar                   = "KUBERNETES_POD_NAME"
	consulPodNameLabelTemplate      = "k8sPodName: %s"
	consulPodNamespaceLabelTemplate = "k8sPodNamespace: %s"
)

// Client is an interface for client to Kubernetes API.
type Client interface {
	// GetPod returns current pod data.
	GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error)
	// GetFailureDomainTags returns current failure domain for pod
	GetFailureDomainTags(ctx context.Context, pod *corev1.Pod) ([]string, error)
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

func (c *defaultClient) GetFailureDomainTags(ctx context.Context, pod *corev1.Pod) ([]string, error) {
	client, err := k8s.NewInClusterClient()
	if err != nil {
		return nil, fmt.Errorf("unable to create k8s client: %s", err)
	}

	node := &corev1.Node{}
	if err := client.Get(ctx, "", pod.GetSpec().GetNodeName(), node); err != nil {
		return nil, fmt.Errorf("unable to get node data from API: %s", err)
	}
	labels := node.GetMetadata().GetLabels()
	var tags []string
	for k, v := range labels {
		if strings.Contains(k, "failure-domain.beta.kubernetes.io") {
			tags = append(tags, fmt.Sprintf("%s:%s", strings.Split(k, "/")[1], v))
		}
	}

	if len(tags) < 1 {
		return nil, fmt.Errorf("failure domain labels don't exist")
	}
	return tags, nil
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
	host := pod.GetStatus().GetPodIP()
	port := int(*container.Ports[0].ContainerPort)

	service := consul.ServiceInstance{
		ID:    fmt.Sprintf("%s_%d", host, port),
		Name:  serviceName,
		Host:  host,
		Port:  port,
		Check: ConvertToConsulCheck(container.LivenessProbe, host),
	}

	failureDomainTags, err := client.GetFailureDomainTags(ctx, pod)
	if err != nil {
		log.Printf("Won't include failure domain data in registration: %s", err)
	} else {
		service.Tags = append(service.Tags, failureDomainTags...)
	}
	if podName != "" && podNamespace != "" {
		service.Tags = append(service.Tags, fmt.Sprintf(consulPodNameLabelTemplate, podName))
		service.Tags = append(service.Tags, fmt.Sprintf(consulPodNamespaceLabelTemplate, podNamespace))
	}

	labels := pod.GetMetadata().GetLabels()

	for key, value := range labels {
		if value == "tag" {
			service.Tags = append(service.Tags, key)
		}
	}

	return []consul.ServiceInstance{service}, nil
}

func (p *ServiceProvider) client() Client {
	if p.Client != nil {
		return p.Client
	}
	return &defaultClient{}
}
