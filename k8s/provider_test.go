package k8s

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"testing"

	"time"

	"github.com/allegro/consul-registration-hook/consul"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func TestIfFailsIfKubernetesAPIFails(t *testing.T) {
	client := &MockClient{}
	client.client.On("GetPod", context.Background(), "", "").
		Return(nil, errors.New("error"))

	provider := ServiceProvider{
		Client:  client,
		Timeout: 2 * time.Second,
	}

	services, err := provider.Get(context.Background())

	require.Error(t, err)
	require.Empty(t, services)
}

func TestIfReturnsEmptySliceToPodIsNotLabelledCorrectly(t *testing.T) {
	pod := testPod()

	client := &MockClient{}
	client.client.On("GetPod", context.Background(), "", "").
		Return(pod, nil).Once()

	provider := ServiceProvider{
		Client:  client,
		Timeout: 1 * time.Second,
	}

	services, err := provider.Get(context.Background())

	require.NoError(t, err)
	require.Empty(t, services)
}

func TestIfReturnsServiceToRegisterIfAbleToCallKubernetesAPI(t *testing.T) {
	os.Setenv(servicePortEnv, "31011")
	defer os.Unsetenv(servicePortEnv)

	containerName := "name"
	podIP := "192.0.2.2"

	port := int32(8080)
	pod := testPod()
	pod.ObjectMeta.Labels[consulLabelKey] = "serviceName"
	pod.Status.PodIP = podIP
	pod.Spec.Containers[0].Name = containerName
	pod.Spec.Containers[0].Ports = append(pod.Spec.Containers[0].Ports, corev1.ContainerPort{ContainerPort: port})

	client := getMockedClient(pod)
	provider := ServiceProvider{
		Client:  client,
		Timeout: 1 * time.Second,
	}

	services, err := provider.Get(context.Background())

	require.NoError(t, err)
	require.Len(t, services, 1)
	client.client.AssertExpectations(t)

	service := services[0]

	assert.True(t, stringInSlice("instance:podName_8080", service.Tags))
	assert.True(t, stringInSlice("service-port:31011", service.Tags))
	assert.Equal(t, "192.0.2.2_8080", service.ID)
	assert.Equal(t, "serviceName", service.Name)
	assert.Equal(t, 8080, service.Port)
}

func TestIfReturnsServiceToRegisterWhenLabeledContainerIsSpecified(t *testing.T) {
	containerName := "name"
	podIP := "192.0.2.2"

	appContainerIndex := 0
	envoyContainerIndex := 1

	appContainerPort := int32(8080)
	sidecarContainerPort := int32(8181)
	pod := testPod()
	pod.ObjectMeta.Labels[consulLabelKey] = "serviceName"
	pod.ObjectMeta.Labels[consulRegisterLabelKey] = "sidecar"
	pod.Status.PodIP = podIP
	pod.Spec.Containers[appContainerIndex].Name = containerName
	pod.Spec.Containers[appContainerIndex].Ports = append(pod.Spec.Containers[appContainerIndex].Ports, corev1.ContainerPort{ContainerPort: appContainerPort})
	pod.Spec.Containers[envoyContainerIndex].Ports = append(pod.Spec.Containers[envoyContainerIndex].Ports, corev1.ContainerPort{ContainerPort: sidecarContainerPort})

	client := getMockedClient(pod)
	provider := ServiceProvider{
		Client:  client,
		Timeout: 1 * time.Second,
	}

	services, err := provider.Get(context.Background())

	require.NoError(t, err)
	require.Len(t, services, 1)
	client.client.AssertExpectations(t)

	service := services[0]

	assert.Equal(t, []string{"instance:podName_8181"}, service.Tags)
	assert.Equal(t, "192.0.2.2_8181", service.ID)
	assert.Equal(t, "serviceName", service.Name)
	assert.Equal(t, 8181, service.Port)
}

func TestIfReturnsServiceToRegisterWhenPortDefinitionsIsDefined(t *testing.T) {
	setEnv(t, "testdata/port_definitions_multiple_tags_vip_and_consul.json")
	os.Setenv(servicePortEnv, "31011")
	defer os.Unsetenv(servicePortEnv)

	defer unsetEnv(t)
	containerName := "name"
	podIP := "192.0.2.2"

	appContainerIndex := 0
	envoyContainerIndex := 1

	appContainerPort := int32(8080)
	sidecarContainerPort := int32(8181)
	pod := testPod()
	pod.ObjectMeta.Labels[consulLabelKey] = "serviceName"
	pod.ObjectMeta.Labels[consulRegisterLabelKey] = "sidecar"
	pod.Status.PodIP = podIP
	pod.Spec.Containers[appContainerIndex].Name = containerName
	pod.Spec.Containers[appContainerIndex].Ports = append(pod.Spec.Containers[appContainerIndex].Ports, corev1.ContainerPort{ContainerPort: appContainerPort})
	pod.Spec.Containers[envoyContainerIndex].Ports = append(pod.Spec.Containers[envoyContainerIndex].Ports, corev1.ContainerPort{ContainerPort: sidecarContainerPort})
	pod.ObjectMeta.Annotations = map[string]string{
		"CONSUL_TAG_0": "tag-x:tag-x",
		"CONSUL_TAG_1": "compute-type:k8s",
		"CONSUL_TAG_2": "tag-y",
		"CONSUL_TAG_3": "default-monitoring",
		"CONSUL_TAG_4": "tag-z",
	}

	client := getMockedClient(pod)
	provider := ServiceProvider{
		Client:  client,
		Timeout: 1 * time.Second,
	}

	services, err := provider.Get(context.Background())

	require.NoError(t, err)
	require.Len(t, services, 3)
	client.client.AssertExpectations(t)

	service := services[0]
	assert.True(t, stringInSlice("instance:podName_31011", service.Tags))
	assert.True(t, stringInSlice("compute-type:k8s", service.Tags))
	assert.True(t, stringInSlice("service-port:31011", service.Tags))
	assert.True(t, stringInSlice("tag-y", service.Tags))
	assert.True(t, stringInSlice("default-monitoring", service.Tags))
	assert.True(t, stringInSlice("tag-x:tag-x", service.Tags))
	assert.True(t, stringInSlice("tag-z", service.Tags))
	assert.False(t, stringInSlice("vip:vip.example.com_50000", service.Tags))
	assert.Equal(t, "192.0.2.2_31011", service.ID)
	assert.Equal(t, "simple-service", service.Name)
	assert.Equal(t, 31011, service.Port)

	service = services[1]
	assert.True(t, stringInSlice("instance:podName_31012", service.Tags))
	assert.True(t, stringInSlice("compute-type:k8s", service.Tags))
	assert.True(t, stringInSlice("service-port:31000", service.Tags))
	assert.True(t, stringInSlice("tag-y", service.Tags))
	assert.True(t, stringInSlice("default-monitoring", service.Tags))
	assert.True(t, stringInSlice("tag-x:tag-x", service.Tags))
	assert.True(t, stringInSlice("tag-z", service.Tags))
	assert.Equal(t, "192.0.2.2_31012", service.ID)
	assert.Equal(t, "extra-service-without-vip", service.Name)
	assert.Equal(t, 31012, service.Port)

	service = services[2]
	assert.True(t, stringInSlice("instance:podName_31013", service.Tags))
	assert.True(t, stringInSlice("compute-type:k8s", service.Tags))
	assert.True(t, stringInSlice("service-port:31011", service.Tags))
	assert.True(t, stringInSlice("tag-y", service.Tags))
	assert.True(t, stringInSlice("default-monitoring", service.Tags))
	assert.True(t, stringInSlice("tag-z", service.Tags))
	assert.True(t, stringInSlice("vip:vip.example.com_50000", service.Tags))
	assert.True(t, stringInSlice("tag-x:tag-x", service.Tags))
	assert.Equal(t, "192.0.2.2_31013", service.ID)
	assert.Equal(t, "extra-service-with-vip", service.Name)
	assert.Equal(t, 31013, service.Port)

}

func TestIfReturnsServiceToRegisterWhenPortDefinitionsAndLbaasIsDefined(t *testing.T) {
	setEnv(t, "testdata/port_definitions_lbaas.json")
	os.Setenv(servicePortEnv, "31010")
	defer os.Unsetenv(servicePortEnv)

	defer unsetEnv(t)
	containerName := "name"
	podIP := "192.0.2.2"

	appContainerIndex := 0
	envoyContainerIndex := 1

	appContainerPort := int32(8080)
	sidecarContainerPort := int32(8181)
	pod := testPod()
	pod.ObjectMeta.Labels[consulLabelKey] = "serviceName"
	pod.ObjectMeta.Labels[consulRegisterLabelKey] = "sidecar"
	pod.Status.PodIP = podIP
	pod.Spec.Containers[appContainerIndex].Name = containerName
	pod.Spec.Containers[appContainerIndex].Ports = append(pod.Spec.Containers[appContainerIndex].Ports, corev1.ContainerPort{ContainerPort: appContainerPort})
	pod.Spec.Containers[envoyContainerIndex].Ports = append(pod.Spec.Containers[envoyContainerIndex].Ports, corev1.ContainerPort{ContainerPort: sidecarContainerPort})
	pod.ObjectMeta.Annotations = map[string]string{
		"CONSUL_TAG_0": "tag-x:tag-x",
		"CONSUL_TAG_1": "tag-y",
		"CONSUL_TAG_2": "compute-type:k8s",
		"CONSUL_TAG_3": "default-monitoring",
		"CONSUL_TAG_4": "lbaas:service.example.com_80",
	}

	client := getMockedClient(pod)
	provider := ServiceProvider{
		Client:  client,
		Timeout: 1 * time.Second,
	}

	services, err := provider.Get(context.Background())

	require.NoError(t, err)
	require.Len(t, services, 2)
	client.client.AssertExpectations(t)

	service := services[0]
	assert.True(t, stringInSlice("instance:podName_31010", service.Tags), "missing tag instance:podName_31010")
	assert.True(t, stringInSlice("compute-type:k8s", service.Tags), "missing tag compute-type:k8s")
	assert.True(t, stringInSlice("service-port:31010", service.Tags), "missing tag service-port:31010")
	assert.True(t, stringInSlice("envoy", service.Tags), "missing tag envoy")
	assert.True(t, stringInSlice("tag-x:tag-x", service.Tags), "missing tag tag-x:tag-x")
	assert.True(t, stringInSlice("tag-y", service.Tags), "missing tag tag-y")
	assert.True(t, stringInSlice("default-monitoring", service.Tags), "missing tag default-monitoring")
	assert.True(t, stringInSlice("lbaas:service.example.com_80", service.Tags), "missing tag lbaas:service.example.com_80")
	assert.Equal(t, "192.0.2.2_31010", service.ID)
	assert.Equal(t, "simple-service", service.Name)
	assert.Equal(t, 31010, service.Port)

	service = services[1]
	assert.True(t, stringInSlice("instance:podName_31011", service.Tags), "missing tag instance:podName_31011")
	assert.True(t, stringInSlice("compute-type:k8s", service.Tags), "missing tag compute-type:k8s")
	assert.False(t, stringInSlice("envoy", service.Tags), "missing tag envoy")
	assert.True(t, stringInSlice("tag-x:tag-x", service.Tags), "missing tag tag-x:tag-x")
	assert.True(t, stringInSlice("tag-y", service.Tags), "missing tag tag-y")
	assert.True(t, stringInSlice("default-monitoring", service.Tags), "missing tag default-monitoring")
	assert.False(t, stringInSlice("lbaas:service.example.com_80", service.Tags), "unwanted tag lbaas:service.example.com_80")
	assert.Equal(t, "192.0.2.2_31011-secured", service.ID)
	assert.Equal(t, "simple-service-secured", service.Name)
	assert.Equal(t, 31011, service.Port)
}

func TestIfReturnsServiceToRegisterAppPortWhenNoLabelSpecified(t *testing.T) {
	containerName := "name"
	podIP := "192.0.2.2"

	appContainerIndex := 0
	envoyContainerIndex := 1

	appContainerPort := int32(8080)
	sidecarContainerPort := int32(8181)
	pod := testPod()
	pod.ObjectMeta.Labels[consulLabelKey] = "serviceName"
	pod.Status.PodIP = podIP
	pod.Spec.Containers[appContainerIndex].Name = containerName
	pod.Spec.Containers[appContainerIndex].Ports = append(pod.Spec.Containers[appContainerIndex].Ports, corev1.ContainerPort{ContainerPort: appContainerPort})
	pod.Spec.Containers[envoyContainerIndex].Ports = append(pod.Spec.Containers[envoyContainerIndex].Ports, corev1.ContainerPort{ContainerPort: sidecarContainerPort})

	client := getMockedClient(pod)
	provider := ServiceProvider{
		Client:  client,
		Timeout: 1 * time.Second,
	}

	services, err := provider.Get(context.Background())

	require.NoError(t, err)
	require.Len(t, services, 1)
	client.client.AssertExpectations(t)

	service := services[0]

	assert.Equal(t, []string{"instance:podName_8080"}, service.Tags)
	assert.Equal(t, "192.0.2.2_8080", service.ID)
	assert.Equal(t, "serviceName", service.Name)
	assert.Equal(t, 8080, service.Port)
}

func TestIfReturnsErrorWhenNoPortsDefined(t *testing.T) {
	containerName := "name"
	podIP := "192.0.2.2"

	appContainerIndex := 0

	pod := testPod()
	pod.ObjectMeta.Labels[consulLabelKey] = "serviceName"
	pod.ObjectMeta.Labels[consulRegisterLabelKey] = "sidecar"
	pod.Status.PodIP = podIP
	pod.Spec.Containers[appContainerIndex].Name = containerName

	client := getMockedClient(pod)
	provider := ServiceProvider{
		Client:  client,
		Timeout: 1 * time.Second,
	}

	_, err := provider.Get(context.Background())
	require.EqualError(t, err, "unable to register, cannot find containerPort")
}

func TestIfFailsWhenUnableToDetermineIP(t *testing.T) {
	client := &MockClient{}

	podWithoutIP := composeTestCasePod(nil)
	emptyIP := ""
	podWithoutIP.Status.PodIP = emptyIP
	client.client.On("GetPod", context.Background(), "", "").
		Return(podWithoutIP, nil)

	client.client.On("GetFailureDomainTags", context.Background(), podWithoutIP).
		Return(nil, nil).Once()

	provider := ServiceProvider{
		Client:  client,
		Timeout: 1 * time.Second,
	}

	services, err := provider.Get(context.Background())

	require.Error(t, err)
	require.Empty(t, services)
}

func TestIfRetriesWhenInitialIPEmpty(t *testing.T) {
	client := &MockClient{}

	podWithoutIP := composeTestCasePod(nil)
	emptyIP := ""
	podWithoutIP.Status.PodIP = emptyIP
	client.client.On("GetPod", context.Background(), "", "").
		Return(podWithoutIP, nil).Times(3)

	podWithIP := composeTestCasePod(nil)
	client.client.On("GetPod", context.Background(), "", "").
		Return(podWithIP, nil).Once()

	client.client.On("GetFailureDomainTags", context.Background(), mock.Anything).
		Return(nil, nil).Once()

	provider := ServiceProvider{
		Client:  client,
		Timeout: 10 * time.Second,
	}

	services, err := provider.Get(context.Background())

	require.NoError(t, err)
	require.Len(t, services, 1)
	client.client.AssertExpectations(t)

	service := services[0]

	assert.Equal(t, []string{"instance:podName_8080"}, service.Tags)
	assert.Equal(t, "192.0.2.2_8080", service.ID)
	assert.Equal(t, "serviceName", service.Name)
	assert.Equal(t, 8080, service.Port)
}

var labelsAndAnnotationsTestCases = []struct {
	pod                *corev1.Pod
	expectedConsulTags []string
}{
	{
		pod: composeTestCasePod(
			map[string]string{
				"CONSUL_TAG_0": "KEY0: VALUE0"}),
		expectedConsulTags: []string{"KEY0: VALUE0", "instance:podName_8080"},
	},
	{
		pod: composeTestCasePod(
			map[string]string{
				"CONSUL_TAG_0": "KEY0: VALUE0",
				"CONSUL_TAG_1": ""}),
		expectedConsulTags: []string{"KEY0: VALUE0", "instance:podName_8080"},
	},
	{
		pod: composeTestCasePod(
			map[string]string{
				"CONSUL_TAG_0": "KEY0: VALUE0",
				"CONSUL_TAG_1": "KEY1: VALUE1"}),
		expectedConsulTags: []string{"KEY0: VALUE0", "KEY1: VALUE1", "instance:podName_8080"},
	},
	{
		pod: composeTestCasePod(
			map[string]string{
				"CONSUL_TAG_0":   "KEY0: VALUE0",
				"CONSUL_TAG_1":   "KEY1: VALUE1",
				"CONSUL_TAG_1_a": "KEY2: VALUE2",
			}),
		expectedConsulTags: []string{"KEY0: VALUE0", "KEY1: VALUE1", "KEY2: VALUE2", "instance:podName_8080"},
	},
	{
		pod: composeTestCasePod(
			map[string]string{
				"CONSUL_TAG_0":   "KEY0: VALUE0",
				"CONSUL_TAG_1":   "KEY0: VALUE0",
				"CONSUL_TAG_1_a": "KEY2: VALUE2",
			}),
		expectedConsulTags: []string{"KEY0: VALUE0", "KEY0: VALUE0", "KEY2: VALUE2", "instance:podName_8080"},
	},
}

func composeTestCasePod(annotations map[string]string) *corev1.Pod {
	port := int32(8080)
	containerName := "name"
	pod := testPod()
	pod.ObjectMeta.Annotations = annotations
	pod.ObjectMeta.Labels[consulLabelKey] = "serviceName"
	pod.Spec.Containers[0].Name = containerName
	pod.Spec.Containers[0].Ports = append(pod.Spec.Containers[0].Ports, corev1.ContainerPort{ContainerPort: port})

	return pod
}

func TestLabelsAndAnnotationsToConsulTagsConversion(t *testing.T) {
	for _, testCase := range labelsAndAnnotationsTestCases {
		client := getMockedClient(testCase.pod)
		provider := ServiceProvider{
			Client:  client,
			Timeout: 1 * time.Second,
		}

		services, err := provider.Get(context.Background())

		require.NoError(t, err)
		client.client.AssertExpectations(t)
		require.Len(t, services, 1)
		service := services[0]
		assert.ElementsMatch(t, service.Tags, testCase.expectedConsulTags)
	}
}

func getMockedClient(pod *corev1.Pod) *MockClient {
	client := &MockClient{}
	client.client.On("GetPod", context.Background(), "", "").
		Return(pod, nil)
	client.client.On("GetFailureDomainTags", context.Background(), pod).
		Return(nil, nil).Once()
	return client
}

func TestIfConvertNodeFailureDomainTagsToConsulTags(t *testing.T) {
	containerName := "name"
	podIP := "192.0.2.2"

	appContainerIndex := 0
	envoyContainerIndex := 1

	appContainerPort := int32(8080)
	sidecarContainerPort := int32(8181)
	pod := testPod()
	pod.ObjectMeta.Labels[consulLabelKey] = "serviceName"
	pod.ObjectMeta.Labels[consulRegisterLabelKey] = "sidecar"
	pod.Status.PodIP = podIP
	pod.Spec.Containers[appContainerIndex].Name = containerName
	pod.Spec.Containers[appContainerIndex].Ports = append(pod.Spec.Containers[appContainerIndex].Ports, corev1.ContainerPort{ContainerPort: appContainerPort})
	pod.Spec.Containers[envoyContainerIndex].Ports = append(pod.Spec.Containers[envoyContainerIndex].Ports, corev1.ContainerPort{ContainerPort: sidecarContainerPort})
	pod.Spec.NodeName = "testNode"

	node := testNode()
	client := defaultClient{k8sClient: testclient.NewSimpleClientset()}
	_, err := client.k8sClient.CoreV1().Nodes().Create(context.Background(), node, metav1.CreateOptions{})

	tags, err := client.GetFailureDomainTags(context.Background(), pod)
	fmt.Printf("%v", tags)
	require.NoError(t, err)
	assert.Contains(t, tags, "region:region1")
	assert.Contains(t, tags, "zone:zone1")
}

func testPod() *corev1.Pod {
	podIP := "192.0.2.2"
	podName := "podName"

	return &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Ports: []corev1.ContainerPort{},
				},
				{
					Name:  "sidecar",
					Ports: []corev1.ContainerPort{},
				},
			},
		},
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
			Name:        podName,
		},
		Status: corev1.PodStatus{
			PodIP: podIP,
		},
	}
}

func testPodWithProbe() *corev1.Pod {
	podIP := "192.0.2.2"
	podName := "podName"
	return &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Ports: []corev1.ContainerPort{},
					ReadinessProbe: &corev1.Probe{
						Handler: corev1.Handler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/status/ping",
								Port: intstr.FromInt(3333),
							},
						},
					},
					StartupProbe: &corev1.Probe{
						Handler: corev1.Handler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/status/ping",
								Port: intstr.FromInt(3333),
							},
						},
					},
					LivenessProbe: &corev1.Probe{
						Handler: corev1.Handler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/status/ping",
								Port: intstr.FromInt(3333),
							},
						},
					},
				},
				{
					Name:  "sidecar",
					Ports: []corev1.ContainerPort{},
				},
			},
		},
		Status: corev1.PodStatus{
			PodIP: podIP,
		},
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
			Name:        podName,
		},
	}
}

func testNode() *corev1.Node {
	labels := make(map[string]string)
	labels["failure-domain.beta.kubernetes.io/region"] = "region1"
	labels["failure-domain.beta.kubernetes.io/zone"] = "zone1"

	return &corev1.Node{
		Spec:   corev1.NodeSpec{},
		Status: corev1.NodeStatus{},
		ObjectMeta: metav1.ObjectMeta{
			Name:   "testNode",
			Labels: labels,
		},
	}
}

type MockClient struct {
	client    mock.Mock
	k8sClient mock.Mock
}

func (c *MockClient) GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error) {
	args := c.client.Called(ctx, namespace, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*corev1.Pod), args.Error(1)
}

func (c *MockClient) GetFailureDomainTags(ctx context.Context, pod *corev1.Pod) ([]string, error) {
	args := c.client.Called(ctx, pod)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).([]string), args.Error(1)
}

func (c *MockClient) DoProbeCheck(pr *corev1.Probe, ip string) error {
	args := c.client.Called(pr, ip)
	if args.Get(0) == nil {
		return args.Error(0)
	}
	return args.Error(0)
}

func TestGenerateServicesWithProperHealthCheck(t *testing.T) {
	setEnv(t, "testdata/port_definitions_probe_and_service_only.json")
	defer unsetEnv(t)

	pod := testPod()
	path := "/status/ping"
	pod.Spec.Containers[0].LivenessProbe = &corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: path,
			},
		},
	}
	expectedServices := []consul.ServiceInstance{
		{
			Check: &consul.Check{
				Address: "http://192.0.2.2:0/status/ping",
				Type:    "HTTP_GET",
			},
		},
	}

	services, err := generateServices("serviceName", pod, nil)
	require.NoError(t, err)

	assert.Len(t, services, 1)
	assert.Equal(t, expectedServices[0].Check, services[0].Check)
}

func TestGenerateServicesWithReadinessHealthCheckIfExists(t *testing.T) {
	setEnv(t, "testdata/port_definitions_probe_and_service_only.json")
	defer unsetEnv(t)

	pod := testPod()
	pod.Spec.Containers[0].LivenessProbe = &corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/status/ping",
			},
		},
	}
	pod.Spec.Containers[0].ReadinessProbe = &corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/status/ready",
			},
		},
	}
	expectedServices := []consul.ServiceInstance{
		{
			Check: &consul.Check{
				Address: "http://192.0.2.2:0/status/ready",
				Type:    "HTTP_GET",
			},
		},
	}

	services, err := generateServices("serviceName", pod, nil)
	require.NoError(t, err)

	assert.Len(t, services, 1)
	assert.Equal(t, expectedServices[0].Check, services[0].Check)
}

func TestCheckOneServiceFromWithProperTags(t *testing.T) {
	setEnv(t, "testdata/port_definitions_probe_and_service_only.json")
	defer unsetEnv(t)

	pod := testPod()
	tags := []string{"a", "b", "c"}
	expectedServices := []consul.ServiceInstance{
		{
			Tags: []string{
				"a", "b", "c", "envoy", "instance:podName_31000",
			},
		},
	}

	services, err := generateServices("serviceName", pod, tags)
	require.NoError(t, err)

	assert.Len(t, services, 1)
	assert.Equal(t, expectedServices[0].Tags, services[0].Tags)
}

func TestShouldCheckMultipleServicesWithGlobalTagsCombinedAndWithPortSpecificTags(t *testing.T) {
	setEnv(t, "testdata/port_definitions_service_probe_secured.json")
	defer unsetEnv(t)
	pod := testPod()
	tags := []string{"a", "b", "c"}
	expectedServices := []consul.ServiceInstance{
		{
			Tags: []string{"a", "b", "c", "instance:podName_31000"},
		},
		{
			Tags: []string{"a", "b", "c", "instance:podName_31002", "secureConnection:true"},
		},
		{
			Tags: []string{"a", "b", "c", "envoy", "frontend:generic-app", "instance:podName_31003", "service-port:31000"},
		},
	}

	services, err := generateServices("serviceName", pod, tags)
	require.NoError(t, err)

	assert.Len(t, services, 3)
	for i, service := range services {
		sort.Strings(service.Tags)
		assert.Equal(t, expectedServices[i].Tags, service.Tags)
	}
}

func TestShouldCheckMultipleServiceNames(t *testing.T) {
	setEnv(t, "testdata/port_definitions_service_probe_secured.json")
	defer unsetEnv(t)

	pod := testPod()
	tags := []string{"a", "b", "c"}
	expectedServices := []consul.ServiceInstance{
		{
			Name: "serviceName",
		},
		{
			Name: "generic-app-secured",
		},
		{
			Name: "generic-app-frontend",
		},
	}

	services, err := generateServices("serviceName", pod, tags)
	require.NoError(t, err)

	assert.Len(t, services, 3)
	for i, service := range services {
		assert.Equal(t, expectedServices[i].Name, service.Name)
	}
}

func TestShouldGenerateSecureServiceAndFirstServiceWithoutTag(t *testing.T) {
	setEnv(t, "testdata/port_definitions_first_service_without_tag.json")
	defer unsetEnv(t)
	pod := testPod()
	tags := []string{"a", "b", "c"}
	expectedServices := []consul.ServiceInstance{
		{
			Name: "serviceName",
		},
		{
			Name: "generic-app-secured",
		},
	}

	services, err := generateServices("serviceName", pod, tags)
	require.NoError(t, err)

	assert.Len(t, services, 2)
	for i, service := range services {
		assert.Equal(t, expectedServices[i].Name, service.Name)
	}
}

func TestShouldGenerateServiceIDsForDeregistration(t *testing.T) {
	servicesForRegistrationSingle := []consul.ServiceInstance{
		{
			ID: "IP_PORT",
			Check: &consul.Check{
				Address: "http://192.0.2.2:0/status/ping",
				Type:    "HTTP_GET",
			},
		},
	}
	servicesForDeregistrationSingle := []consul.ServiceInstance{
		{
			ID: "IP_PORT-secured",
			Check: &consul.Check{
				Address: "http://192.0.2.2:0/status/ping",
				Type:    "HTTP_GET",
			},
		},
	}

	servicesForRegistrationDouble := []consul.ServiceInstance{
		{
			ID: "IP_PORT",
			Check: &consul.Check{
				Address: "http://192.0.2.2:0/status/ping",
				Type:    "HTTP_GET",
			},
		},
		{
			ID: "IP_PORT-secured",
			Check: &consul.Check{
				Address: "http://192.0.2.2:0/status/ping",
				Type:    "HTTP_GET",
			},
		},
	}

	pod := testPod()
	client := getMockedClient(pod)
	provider := ServiceProvider{
		Client:  client,
		Timeout: 1 * time.Second,
	}

	services := provider.GenerateSecured(context.Background(), servicesForRegistrationSingle)
	assert.Len(t, services, 1)
	for i, service := range services {
		assert.Equal(t, servicesForDeregistrationSingle[i].ID, service.ID)
	}

	services = provider.GenerateSecured(context.Background(), servicesForRegistrationDouble)
	assert.Len(t, services, 0)
}

func TestIfPodReturnsProbe(t *testing.T) {
	pod := testPodWithProbe()
	podIP := "192.0.2.2"
	initialDelay := int32(10)

	appContainerIndex := 0

	pod.Status.PodIP = podIP
	pod.Spec.Containers[appContainerIndex].ReadinessProbe.InitialDelaySeconds = initialDelay

	pr := &corev1.Probe{
		InitialDelaySeconds: 10,
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/status/ping",
				Port: intstr.FromInt(3333),
			},
		},
	}

	client := getMockedClient(pod)
	provider := ServiceProvider{
		Client:  client,
		Timeout: 1 * time.Second,
	}

	// probe -> StartupProbe
	probe := provider.getProbe(pod)
	require.NotEqual(t, pr, probe)

	// probe -> ReadinessProbe
	pod.Spec.Containers[0].StartupProbe = nil
	probe = provider.getProbe(pod)
	require.Equal(t, pr, probe)

	// probe -> nil
	pod.Spec.Containers[0].ReadinessProbe = nil
	probe = provider.getProbe(pod)
	require.Nil(t, probe)
}

func TestServiceIsAliveIfPodHasHTTPHandlerProbe(t *testing.T) {
	pod := testPodWithProbe()
	podIP := "127.0.0.1"

	client := &MockClient{}
	provider := ServiceProvider{
		Client:  client,
		Timeout: 1 * time.Second,
	}
	pod.Status.PodIP = podIP
	pr := &corev1.Probe{
		InitialDelaySeconds: 1,
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/status/ping",
				Port: intstr.FromString("3333"),
			},
		},
	}
	client.client.On("DoProbeCheck", pr, podIP).
		Return(errors.New("http error")).Twice().On("DoProbeCheck", pr, podIP).Return(nil)
	provider.checkServiceLiveness(pr, podIP)
	client.client.ExpectedCalls = nil
}

func TestServiceIsAliveIfPodHasTCPSocketProbe(t *testing.T) {

	podIP := "127.0.0.1"
	client := &MockClient{}

	provider := ServiceProvider{
		Client:  client,
		Timeout: 1 * time.Second,
	}
	pr := &corev1.Probe{
		InitialDelaySeconds: 1,
		Handler: corev1.Handler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.FromString("10000"),
			},
		},
	}
	client.client.On("DoProbeCheck", pr, podIP).
		Return(nil)
	provider.checkServiceLiveness(pr, podIP)
}

func TestTerminatingServiceIsFailedToRegister(t *testing.T) {
	pod := testPod()
	now := metav1.NewTime(time.Now())
	pod.ObjectMeta.DeletionTimestamp = &now
	client := &MockClient{}

	provider := ServiceProvider{
		Client:  client,
		Timeout: 1 * time.Second,
	}

	client.client.On("GetPod", context.Background(), "", "").
		Return(pod, nil).Times(3)
	isTerminating, err := provider.IsPodTerminating(context.Background())
	assert.NoError(t, err)
	assert.True(t, isTerminating)
}

func TestTerminatingServiceReturnsErrorAndTerminationTrueOnFailedApiCall(t *testing.T) {
	pod := testPod()
	now := metav1.NewTime(time.Now())
	pod.ObjectMeta.DeletionTimestamp = &now
	client := &MockClient{}

	provider := ServiceProvider{
		Client:  client,
		Timeout: 1 * time.Second,
	}

	client.client.On("GetPod", context.Background(), "", "").
		Return(pod, fmt.Errorf("failed to call k8s api")).Times(3)
	isTerminating, err := provider.IsPodTerminating(context.Background())
	assert.Error(t, err)
	assert.True(t, isTerminating)
}

func TestReturnsOfDoProbeCheckMethod(t *testing.T) {

	clinet := &defaultClient{}
	sp := ServiceProvider{
		Client:             clinet,
		Timeout:            1,
		HealthCheckTimeout: 30,
	}
	ip := "127.0.0.1"
	path := "/status/ping"
	pr := &corev1.Probe{
		InitialDelaySeconds: 1,
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/status/ping",
			},
		},
	}

	testServer := httptest.NewServer(http.HandlerFunc(
		func(res http.ResponseWriter, req *http.Request) {
			switch req.RequestURI {
			case path:
				res.Header().Set("Content-Type", "text/html")
			default:
				http.Error(res, "", http.StatusNotFound)
				return
			}
		},
	),
	)
	defer testServer.Close()

	// matching ports
	_, port, _ := net.SplitHostPort(testServer.Listener.Addr().String())
	pr.Handler.HTTPGet.Port = intstr.FromString(port)
	assert.NoError(t, sp.Client.DoProbeCheck(pr, ip))

	// different ports
	pr.Handler.HTTPGet.Port = intstr.FromString("1111")
	assert.Error(t, sp.Client.DoProbeCheck(pr, ip))

	port = "40000"
	pr = &corev1.Probe{
		InitialDelaySeconds: 1,
		Handler: corev1.Handler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.FromString("40000"),
			},
		},
	}

	l, err := net.Listen("tcp", ip+":"+port)
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		os.Exit(1)
	}
	defer l.Close()

	// matching ports
	assert.NoError(t, sp.Client.DoProbeCheck(pr, ip))

	// different ports
	pr.Handler.TCPSocket.Port = intstr.FromString("10000")
	assert.Error(t, sp.Client.DoProbeCheck(pr, ip))

}

func TestConsulHookGlobalTimeout(t *testing.T) {
	pod := testPodWithProbe()
	podIP := "127.0.0.1"

	client := &MockClient{}
	provider := ServiceProvider{
		Client:             client,
		Timeout:            1 * time.Second,
		HealthCheckTimeout: 5 * time.Second,
	}
	pod.Status.PodIP = podIP
	pr := &corev1.Probe{
		InitialDelaySeconds: 1,
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/status/ping",
				Port: intstr.FromString("3333"),
			},
		},
	}
	client.client.On("DoProbeCheck", pr, podIP).
		Return(errors.New("http error"))
	err := provider.checkServiceLiveness(pr, podIP)
	if assert.Error(t, err) {
		assert.Equal(t, fmt.Sprintf("endpoint not ready: healthcheck timeout: %s", provider.HealthCheckTimeout), err.Error())
	}
	client.client.ExpectedCalls = nil
}
