package k8s

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/allegro/consul-registration-hook/consul"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	consulLabelKey                  = "consul"
	consulRegisterLabelKey          = "consulContainer"
	consulTagPrefix                 = "CONSUL_TAG_"
	podNamespaceEnvVar              = "KUBERNETES_POD_NAMESPACE"
	podNameEnvVar                   = "KUBERNETES_POD_NAME"
	consulPodNameLabelTemplate      = "k8sPodName: %s"
	consulPodNamespaceLabelTemplate = "k8sPodNamespace: %s"
	instanceFormat                  = "instance:%s_%d"
	securedIDPostfix                = "-secured"
	lbaasPrefix                     = "lbaas:"
	servicePortEnv                  = "PORT_SERVICE"
	servicePortTemplate             = "service-port:%s"
)

// Client is an interface for client to Kubernetes API.
type Client interface {
	// GetPod returns current pod data.
	GetPod(ctx context.Context, podNamespace string, podName string) (*corev1.Pod, error)
	// GetFailureDomainTags returns current failure domain for pod
	GetFailureDomainTags(ctx context.Context, pod *corev1.Pod) ([]string, error)
	// DoProbeCheck check if service is alive
	DoProbeCheck(pod *corev1.Probe, ip string) error
}

type defaultClient struct {
	k8sClient kubernetes.Interface
}

func (c *defaultClient) GetPod(ctx context.Context, namespace string, name string) (*corev1.Pod, error) {
	pod, err := c.k8sClient.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to get pod(%s) in namespace (%s) data from API: %s", pod, namespace, err)
	}

	return pod, nil
}

func (c *defaultClient) GetNode(ctx context.Context, nodeName string) (*corev1.Node, error) {
	node, err := c.k8sClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to get pod data from API: %s", err)
	}
	return node, nil
}

func (c *defaultClient) GetFailureDomainTags(ctx context.Context, pod *corev1.Pod) ([]string, error) {
	node, err := c.GetNode(ctx, pod.Spec.NodeName)
	if err != nil {
		return nil, fmt.Errorf("unable to get node data from API: %s", err)
	}
	var tags []string
	for k, v := range node.Labels {
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
	Client             Client
	Timeout            time.Duration
	HealthCheckTimeout time.Duration
}

// GenerateSecured generates list of postfixed Consul services for deregistration
func (p *ServiceProvider) GenerateSecured(ctx context.Context, services []consul.ServiceInstance) []consul.ServiceInstance {
	var NonSecureIDs []string
	var SecureIDs []string
	for _, service := range services {
		if !strings.Contains(service.ID, securedIDPostfix) {
			NonSecureIDs = append(NonSecureIDs, service.ID)
		} else {
			SecureIDs = append(SecureIDs, service.ID)
		}
	}

	var deregisterIDs []string
	for _, service := range NonSecureIDs {
		if !Find(SecureIDs, fmt.Sprintf("%s%s", service, securedIDPostfix)) {
			deregisterIDs = append(deregisterIDs, fmt.Sprintf("%s%s", service, securedIDPostfix))
		}
	}

	var deregisterServices []consul.ServiceInstance
	for _, serviceID := range deregisterIDs {
		srv := consul.ServiceInstance{
			ID: serviceID,
		}
		deregisterServices = append(deregisterServices, srv)
	}
	return deregisterServices
}

func (p *ServiceProvider) IsPodTerminating(ctx context.Context) (bool, error) {
	client, err := p.client()
	if err != nil {
		return true, fmt.Errorf("unable create K8S API client: %s", err)
	}

	podNamespace := os.Getenv(podNamespaceEnvVar)
	podName := os.Getenv(podNameEnvVar)
	pod, err := client.GetPod(ctx, podNamespace, podName)
	if err != nil {
		return true, err
	}
	// inspired by kubectl: https://github.com/kubernetes/kubernetes/blob/v1.2.0/pkg/kubectl/resource_printer.go#L561-L590
	if pod.DeletionTimestamp != nil {
		return true, nil
	}
	return false, nil
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

	serviceName := pod.GetObjectMeta().GetLabels()[consulLabelKey]
	if serviceName == "" {
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
	//annotations := pod.GetMetadata().GetAnnotations()
	for key, value := range pod.Annotations {
		if strings.HasPrefix(key, consulTagPrefix) && len(value) > 0 {
			globalTags = append(globalTags, value)
		}
	}

	return generateServices(serviceName, pod, globalTags)
}

// client returns kubernetes clientset
func (p *ServiceProvider) client() (Client, error) {
	if p.Client != nil {
		return p.Client, nil
	}
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("couldn't initialize client: %s", err)
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("couldn't initialize client: %s", err)
	}
	return &defaultClient{
		k8sClient: clientset,
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

				if pod.Status.PodIP != "" {
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

func (p *ServiceProvider) CheckProbe(ctx context.Context) error {
	client, err := p.client()
	if err != nil {
		return fmt.Errorf("unable create K8S API client: %s", err)
	}
	podNamespace := os.Getenv(podNamespaceEnvVar)
	podName := os.Getenv(podNameEnvVar)

	pod, err := client.GetPod(ctx, podNamespace, podName)
	if err != nil {
		return fmt.Errorf("unable to get pod data from API: %s", err)
	}
	probe := p.getProbe(pod)
	if probe != nil {
		if err := p.checkServiceLiveness(probe, pod.Status.PodIP); err != nil {
			return err
		}
		return nil
	}
	return nil
}

func (p *ServiceProvider) getProbe(pod *corev1.Pod) *corev1.Probe {
	container := pod.Spec.Containers[0]
	if container.StartupProbe != nil {
		return container.StartupProbe
	}
	if container.ReadinessProbe != nil {
		return container.ReadinessProbe
	}
	return nil
}

func getPortFromProbe(probe *corev1.Probe) string {
	if probe.HTTPGet != nil {
		return probe.HTTPGet.Port.String()
	} else if probe.TCPSocket != nil {
		return probe.TCPSocket.Port.String()
	}
	return ""
}

func getHTTPSchemaFromProbe(probe *corev1.Probe) string {
	if probe.HTTPGet.Scheme != "" {
		return "http"
	}
	if probe.HTTPGet.Scheme == "HTTPS" {
		return "https"
	}
	return "http"
}

func (c *defaultClient) DoProbeCheck(probe *corev1.Probe, podIP string) error {
	port := getPortFromProbe(probe)

	if probe.HTTPGet != nil {
		schema := getHTTPSchemaFromProbe(probe)
		path := probe.HTTPGet.Path
		url := fmt.Sprintf("%s://%s:%s%s", schema, podIP, port, path)
		return doHTTPCheck(url)
	} else if probe.TCPSocket != nil {
		return doTCPCheck(podIP, port)
	}
	return nil
}

func (p *ServiceProvider) getInitialDelay(probe *corev1.Probe) time.Duration {
	if probe.InitialDelaySeconds != 0 {
		return time.Duration(probe.InitialDelaySeconds) * time.Second
	}
	return time.Second * 0
}

func (p *ServiceProvider) getPeriod(probe *corev1.Probe) time.Duration {
	if probe.PeriodSeconds != 0 {
		return time.Duration(probe.PeriodSeconds) * time.Second
	}
	return time.Second * 1
}

func (p *ServiceProvider) checkServiceLiveness(pr *corev1.Probe, podIP string) error {
	cli, err := p.client()
	if err != nil {
		return fmt.Errorf("unable create K8S API client: %s", err)
	}

	initialDelay := p.getInitialDelay(pr)
	period := p.getPeriod(pr)

	log.Printf("witing until endpoint should be ready: %s", initialDelay)
	time.Sleep(initialDelay)

	ch := make(chan bool, 1)
	finished := false
	go func() {
		for !finished {
			err := cli.DoProbeCheck(pr, podIP)
			if err != nil {
				log.Printf("endpoint not ready: %s", err)
			} else {
				ch <- true
			}
			time.Sleep(period)
		}
	}()
	select {
	case <-ch:
		finished = true
		close(ch)
		return nil
	case <-time.After(p.HealthCheckTimeout):
		finished = true
		close(ch)
		return fmt.Errorf("endpoint not ready: healthcheck timeout: %s", p.HealthCheckTimeout)
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

	podName := pod.Name
	host := pod.Status.PodIP
	port := int(container.Ports[0].ContainerPort)
	probe := container.LivenessProbe
	if container.ReadinessProbe != nil {
		probe = container.ReadinessProbe
	}

	service := consul.ServiceInstance{
		ID:    fmt.Sprintf("%s_%d", host, port),
		Name:  serviceName,
		Host:  host,
		Port:  port,
		Check: ConvertToConsulCheck(probe, host),
	}
	service.Tags = make([]string, 0, len(globalTags)+2)
	service.Tags = append(service.Tags, globalTags...)

	servicePort := os.Getenv(servicePortEnv)
	if servicePort != "" {
		service.Tags = append(service.Tags, fmt.Sprintf("service-port:%s", servicePort))
	}
	service.Tags = append(service.Tags, createInstanceTag(podName, port))

	return []consul.ServiceInstance{service}, nil
}

func getContainerToRegister(pod *corev1.Pod) (*corev1.Container, error) {
	var containerToRegister *corev1.Container
	containerToRegisterName, containerDefined := pod.GetObjectMeta().GetLabels()[consulRegisterLabelKey]

	for _, container := range pod.Spec.Containers {
		if container.Name == containerToRegisterName && len(container.Ports) > 0 {
			containerToRegister = &container
			break
		} else if !containerDefined && len(container.Ports) > 0 {
			containerToRegister = &container
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
	host := pod.Status.PodIP
	podName := pod.Name
	// TODO(tz) - provide container id with readiness probe
	container := pod.Spec.Containers[0]

	for idx, portDefinition := range *(portDefinitions) {
		labeledServiceName := portDefinition.labelForConsul()
		if labeledServiceName != "" {
			serviceName = labeledServiceName
		}
		isSecureService := false

		if portDefinition.isService() || labeledServiceName != "" || (idx == 0 && !portDefinitions.HasServicePortDefined()) {
			id := fmt.Sprintf("%s_%d", host, portDefinition.Port)
			if strings.Contains(serviceName, securedIDPostfix) {
				id = fmt.Sprintf("%s_%d%s", host, portDefinition.Port, securedIDPostfix)
				isSecureService = true
			}
			probe := container.LivenessProbe
			if container.ReadinessProbe != nil {
				probe = container.ReadinessProbe
			}
			service := consul.ServiceInstance{
				ID:    id,
				Name:  serviceName,
				Host:  host,
				Port:  portDefinition.Port,
				Check: ConvertToConsulCheck(probe, host),
			}
			service.Tags = make([]string, 0, len(portDefinition.getTags())+len(globalTags)+2)
			if isSecureService {
				for _, globalTag := range globalTags {
					if !strings.HasPrefix(globalTag, lbaasPrefix) {
						service.Tags = append(service.Tags, globalTag)
					}
				}
			} else {
				service.Tags = append(service.Tags, globalTags...)
			}
			service.Tags = append(service.Tags, portDefinition.getTags()...)
			service.Tags = append(service.Tags, createInstanceTag(podName, portDefinition.Port))
			servicePort := os.Getenv(servicePortEnv)
			if servicePort != "" && !stringInSlice(fmt.Sprintf(servicePortTemplate, ""), service.Tags) {
				service.Tags = append(service.Tags, fmt.Sprintf(servicePortTemplate, servicePort))
			}

			services = append(services, service)
		} else if portDefinition.isProbe() {
			// probe is taken from liveness probe of first container, it is assumet to match this one
			// TODO(tz) - compare port with liveness probe
			continue
		}
	}
	return services, nil
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if strings.Contains(b, a) {
			return true
		}
	}
	return false
}

func createInstanceTag(podName string, podPort int) string {
	return fmt.Sprintf(instanceFormat, podName, podPort)
}

// Find takes a slice and looks for an element in it. If found it will
// return it's key, otherwise it will return -1 and a bool of false.
func Find(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}
