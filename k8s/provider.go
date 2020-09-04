package k8s

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/allegro/consul-registration-hook/consul"

	"github.com/ericchiang/k8s"
	corev1 "github.com/ericchiang/k8s/apis/core/v1"
)

const (
	consulLabelKey                  = "consul"
	consulRegisterLabelKey          = "consulContainer"
	consulTagPrefix                 = "CONSUL_TAG_"
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
	podName := os.Getenv(podNameEnvVar)

	pod, err := p.getPodWithRetry(ctx, client, podNamespace, podName)
	if err != nil {
		return nil, fmt.Errorf("unable to get pod data from API: %s", err)
	}

	serviceName, ok := pod.Metadata.Labels[consulLabelKey]
	if !ok {
		return nil, nil
	}

	failureDomainTags, err := client.GetFailureDomainTags(ctx, pod)
	if err != nil {
		log.Printf("Won't include failure domain data in registration: %s", err)
	}
	var globalTags []string

	if podName != "" && podNamespace != "" {
		globalTags = append(globalTags, fmt.Sprintf(consulPodNameLabelTemplate, podName))
		globalTags = append(globalTags, fmt.Sprintf(consulPodNamespaceLabelTemplate, podNamespace))
	}
	globalTags = append(globalTags, failureDomainTags...)

	// annotations allows us to store non alphanumeric values, unlike labels values (alphanumeric, max 63 characters.
	annotations := pod.GetMetadata().GetAnnotations()
	for key, value := range annotations {
		if strings.HasPrefix(key, consulTagPrefix) && len(value) > 0 {
			globalTags = append(globalTags, value)
		}
	}

	return generateServices(serviceName, pod, globalTags)
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
			time.Sleep(time.Second)
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
		return nil, fmt.Errorf("could not get valid Pod data after %s", p.Timeout)
	}
}

func generateServices(serviceName string, pod *corev1.Pod, globalTags []string) ([]consul.ServiceInstance, error) {
	portDefinitions, err := getPortDefinitions()
	if err != nil {
		return nil, err
	}

	if portDefinitions == nil {
		return generateFromContainerPorts(serviceName, pod, globalTags)
	}
	return generateFromPortDefinitions(serviceName, portDefinitions, pod, globalTags)
}

func generateFromContainerPorts(serviceName string, pod *corev1.Pod, globalTags []string) ([]consul.ServiceInstance, error) {
	container, err := getContainerToRegister(pod)
	if err != nil {
		return nil, err
	}

	host := pod.GetStatus().GetPodIP()
	podName := pod.GetMetadata().GetName()
	port := int(*container.Ports[0].ContainerPort)

	service := consul.ServiceInstance{
		ID:    fmt.Sprintf("%s_%d", podName, port),
		Name:  serviceName,
		Host:  host,
		Port:  port,
		Check: ConvertToConsulCheck(container.LivenessProbe, host),
	}
	service.Tags = append(service.Tags, globalTags...)

	return []consul.ServiceInstance{service}, nil
}

func getContainerToRegister(pod *corev1.Pod) (*corev1.Container, error) {
	var containerToRegister *corev1.Container
	containerToRegisterName, containerDefined := pod.Metadata.Labels[consulRegisterLabelKey]

	for _, container := range pod.Spec.Containers {
		if *container.Name == containerToRegisterName && len(container.Ports) > 0 {
			containerToRegister = container
			break
		} else if !containerDefined && len(container.Ports) > 0 {
			containerToRegister = container
			break
		}
	}

	if containerToRegister == nil {
		return nil, fmt.Errorf("unable to register, cannot find containerPort")
	}
	return containerToRegister, nil
}

func generateFromPortDefinitions(serviceName string, portDefinitions *portDefinitions, pod *corev1.Pod, globalTags []string) ([]consul.ServiceInstance, error) {
	var services []consul.ServiceInstance
	host := pod.GetStatus().GetPodIP()
	// TODO(tz) - provide container id with readiness probe
	container := pod.Spec.Containers[0]

	for idx, portDefinition := range *(portDefinitions) {
		labeledServiceName := portDefinition.labelForConsul()
		if labeledServiceName != "" {
			serviceName = labeledServiceName
		}

		if portDefinition.isService() || labeledServiceName != "" || (idx == 0 && !portDefinitions.HasServicePortDefined()) {
			service := consul.ServiceInstance{
				ID:    fmt.Sprintf("%s_%d", host, portDefinition.Port),
				Name:  serviceName,
				Host:  host,
				Port:  portDefinition.Port,
				Check: ConvertToConsulCheck(container.LivenessProbe, host),
				Tags:  globalTags,
			}
			service.Tags = append(service.Tags, portDefinition.getTags()...)
			services = append(services, service)
		} else if portDefinition.isProbe() {
			// probe is taken from liveness probe of first container, it is assumet to match this one
			// TODO(tz) - compare port with liveness probe
			continue
		}
	}
	return services, nil
}
