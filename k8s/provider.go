package k8s

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/wix-playground/consul-registration-hook/consul"

	"github.com/ericchiang/k8s"
	corev1 "github.com/ericchiang/k8s/apis/core/v1"
)

const (
	consulTagPrefix                 = "CONSUL_TAG_"
	podNamespaceEnvVar              = "KUBERNETES_POD_NAMESPACE"
	podNameEnvVar                   = "KUBERNETES_POD_NAME"
	consulSvcEnvVar                 = "CONSUL_SVC"
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
	k8sClient *k8s.Client
}

func (c *defaultClient) GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error) {
	pod := &corev1.Pod{}
	if err := c.k8sClient.Get(ctx, namespace, name, pod); err != nil {
		return nil, fmt.Errorf("unable to get pod data from API: %s", err)
	}

	return pod, nil
}

func (c *defaultClient) GetFailureDomainTags(ctx context.Context, pod *corev1.Pod) ([]string, error) {
	node := &corev1.Node{}
	if err := c.k8sClient.Get(ctx, "", pod.GetSpec().GetNodeName(), node); err != nil {
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
	Client  Client
	Timeout time.Duration
}

// Get returns slice of services that are configured to be registered in Consul
// discovery service.
func (p *ServiceProvider) Get(ctx context.Context) ([]consul.ServiceInstance, error) {
	client, err := p.client()

	if err != nil {
		return nil, fmt.Errorf("unable create K8S API client: %s", err)
	}

	podNamespace := os.Getenv(podNamespaceEnvVar)
	var serviceName = ""
	if os.Getenv(consulSvcEnvVar) != "" {
		serviceName = os.Getenv(consulSvcEnvVar)
	} else {
		return nil, nil
	}
	podName := os.Getenv(podNameEnvVar)

	pod, err := p.getPodWithRetry(ctx, client, podNamespace, podName)
	if err != nil {
		return nil, fmt.Errorf("unable to get pod data from API: %s", err)
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

	// annotations allows us to store non alphanumeric values, unlike labels values (alphanumeric, max 63 characters.
	annotations := pod.GetMetadata().GetAnnotations()
	for key, value := range annotations {
		if strings.HasPrefix(key, consulTagPrefix) && len(value) > 0 {
			service.Tags = append(service.Tags, value)
		}
	}

	return []consul.ServiceInstance{service}, nil
}

func (p *ServiceProvider) client() (Client, error) {
	if p.Client != nil {
		return p.Client, nil
	}
	client, err := k8s.NewInClusterClient()
	if err != nil {
		return nil, fmt.Errorf("couldn't initialize client: %s", err)
	}
	return &defaultClient{
		k8sClient: client,
	}, nil
}

func (p *ServiceProvider) getPodWithRetry(ctx context.Context, client Client, podNamespace, podName string) (pod *corev1.Pod, err error) {
	ch := make(chan *corev1.Pod, 1)
	finished := false
	go func() {
		for !finished {
			pod, err := client.GetPod(ctx, podNamespace, podName)
			if err != nil {
				log.Printf("unable to get pod data from API: %s", err)
			} else {
				if pod.GetStatus().GetPodIP() != "" {
					ch <- pod
				}
			}
			time.Sleep(2 * time.Second)
		}
	}()

	select {
	case res := <-ch:
		finished = true
		close(ch)
		return res, nil
	case <-time.After(p.Timeout):
		finished = true
		close(ch)
		if err != nil {
			return nil, fmt.Errorf("could not get valid Pod data after %s seconds: %s", p.Timeout, err)
		}
		return nil, fmt.Errorf("could not get valid Pod data after %s", p.Timeout)
	}
}
