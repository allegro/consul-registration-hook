package k8s

import (
	"context"
	"errors"
	"fmt"
	"github.com/allegro/consul-registration-hook/consul"
	"github.com/ericchiang/k8s"
	"github.com/ericchiang/k8s/runtime"
	"github.com/golang/protobuf/proto"
	"net/http"
	"net/http/httptest"
	"testing"

	corev1 "github.com/ericchiang/k8s/apis/core/v1"
	metav1 "github.com/ericchiang/k8s/apis/meta/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"time"
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
	containerName := "name"
	podIP := "192.0.2.2"

	port := int32(8080)
	pod := testPod()
	pod.Metadata.Labels[consulLabelKey] = "serviceName"
	pod.Status.PodIP = &podIP
	pod.Spec.Containers[0].Name = &containerName
	pod.Spec.Containers[0].Ports = append(pod.Spec.Containers[0].Ports, &corev1.ContainerPort{ContainerPort: &port})

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

	assert.Len(t, service.Tags, 0)
	assert.Equal(t, "192.0.2.2_8080", service.ID)
	assert.Equal(t, "serviceName", service.Name)
	assert.Equal(t, 8080, service.Port)
}

func TestIfReturnsServiceToRegisterWhenLapeledContainerIsSpecified(t *testing.T) {
	containerName := "name"
	podIP := "192.0.2.2"

	appContainerIndex := 0
	envoyContainerIndex := 1

	appContainerPort := int32(8080)
	sidecarContainerPort := int32(8181)
	pod := testPod()
	pod.Metadata.Labels[consulLabelKey] = "serviceName"
	pod.Metadata.Labels[consulRegisterLabelKey] = "sidecar"
	pod.Status.PodIP = &podIP
	pod.Spec.Containers[appContainerIndex].Name = &containerName
	pod.Spec.Containers[appContainerIndex].Ports = append(pod.Spec.Containers[appContainerIndex].Ports, &corev1.ContainerPort{ContainerPort: &appContainerPort})
	pod.Spec.Containers[envoyContainerIndex].Ports = append(pod.Spec.Containers[envoyContainerIndex].Ports, &corev1.ContainerPort{ContainerPort: &sidecarContainerPort})

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

	assert.Len(t, service.Tags, 0)
	assert.Equal(t, "192.0.2.2_8181", service.ID)
	assert.Equal(t, "serviceName", service.Name)
	assert.Equal(t, 8181, service.Port)
}

func TestIfReturnsServiceToRegisterAppPortWhenNoLabelSpecified(t *testing.T) {
	containerName := "name"
	podIP := "192.0.2.2"

	appContainerIndex := 0
	envoyContainerIndex := 1

	appContainerPort := int32(8080)
	sidecarContainerPort := int32(8181)
	pod := testPod()
	pod.Metadata.Labels[consulLabelKey] = "serviceName"
	pod.Status.PodIP = &podIP
	pod.Spec.Containers[appContainerIndex].Name = &containerName
	pod.Spec.Containers[appContainerIndex].Ports = append(pod.Spec.Containers[appContainerIndex].Ports, &corev1.ContainerPort{ContainerPort: &appContainerPort})
	pod.Spec.Containers[envoyContainerIndex].Ports = append(pod.Spec.Containers[envoyContainerIndex].Ports, &corev1.ContainerPort{ContainerPort: &sidecarContainerPort})

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

	assert.Len(t, service.Tags, 0)
	assert.Equal(t, "192.0.2.2_8080", service.ID)
	assert.Equal(t, "serviceName", service.Name)
	assert.Equal(t, 8080, service.Port)
}

func TestIfReturnsErrorWhenNoPortsDefined(t *testing.T) {
	containerName := "name"
	podIP := "192.0.2.2"

	appContainerIndex := 0

	pod := testPod()
	pod.Metadata.Labels[consulLabelKey] = "serviceName"
	pod.Metadata.Labels[consulRegisterLabelKey] = "sidecar"
	pod.Status.PodIP = &podIP
	pod.Spec.Containers[appContainerIndex].Name = &containerName

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
	podWithoutIP.Status.PodIP = &emptyIP
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
	podWithoutIP.Status.PodIP = &emptyIP
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

	assert.Len(t, service.Tags, 0)
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
		expectedConsulTags: []string{"KEY0: VALUE0"},
	},
	{
		pod: composeTestCasePod(
			map[string]string{
				"CONSUL_TAG_0": "KEY0: VALUE0",
				"CONSUL_TAG_1": ""}),
		expectedConsulTags: []string{"KEY0: VALUE0"},
	},
	{
		pod: composeTestCasePod(
			map[string]string{
				"CONSUL_TAG_0": "KEY0: VALUE0",
				"CONSUL_TAG_1": "KEY1: VALUE1"}),
		expectedConsulTags: []string{"KEY0: VALUE0", "KEY1: VALUE1"},
	},
	{
		pod: composeTestCasePod(
			map[string]string{
				"CONSUL_TAG_0":   "KEY0: VALUE0",
				"CONSUL_TAG_1":   "KEY1: VALUE1",
				"CONSUL_TAG_1_a": "KEY2: VALUE2",
			}),
		expectedConsulTags: []string{"KEY0: VALUE0", "KEY1: VALUE1", "KEY2: VALUE2"},
	},
	{
		pod: composeTestCasePod(
			map[string]string{
				"CONSUL_TAG_0":   "KEY0: VALUE0",
				"CONSUL_TAG_1":   "KEY0: VALUE0",
				"CONSUL_TAG_1_a": "KEY2: VALUE2",
			}),
		expectedConsulTags: []string{"KEY0: VALUE0", "KEY0: VALUE0", "KEY2: VALUE2"},
	},
}

func composeTestCasePod(annotations map[string]string) *corev1.Pod {
	port := int32(8080)
	containerName := "name"
	pod := testPod()
	pod.Metadata.Annotations = annotations
	pod.Metadata.Labels[consulLabelKey] = "serviceName"
	pod.Spec.Containers[0].Name = &containerName
	pod.Spec.Containers[0].Ports = append(pod.Spec.Containers[0].Ports, &corev1.ContainerPort{ContainerPort: &port})

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
	pod := testPod()
	pod.Spec.NodeName = k8s.String("testNode")
	node := testNode()

	data, err := marshalPB(node)

	if err != nil {
		t.Errorf("Test failed due to marshalling: %s", err)
	}

	// Create a test server
	testServer := httptest.NewServer(http.HandlerFunc(
		func(res http.ResponseWriter, req *http.Request) {
			switch req.RequestURI {
			case "/api/v1/nodes/testNode":
				res.Header().Set("Content-Type", "application/vnd.kubernetes.protobuf")
				res.Write(data)
			default:
				http.Error(res, "", http.StatusNotFound)
				return
			}
		},
	),
	)
	defer testServer.Close()

	client := &defaultClient{k8sClient: NewTestK8sClient(testServer.URL)}

	tags, err := client.GetFailureDomainTags(context.Background(), pod)
	require.NoError(t, err)
	assert.Contains(t, tags, "region:region1")
	assert.Contains(t, tags, "zone:zone1")
}

func testPod() *corev1.Pod {
	podIP := "192.0.2.2"

	return &corev1.Pod{
		Spec: &corev1.PodSpec{
			Containers: []*corev1.Container{
				{
					Ports: []*corev1.ContainerPort{},
				},
				{
					Name:  k8s.String("sidecar"),
					Ports: []*corev1.ContainerPort{},
				},
			},
		},
		Status: &corev1.PodStatus{
			PodIP: &podIP,
		},
		Metadata: &metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		},
	}
}

func testNode() *corev1.Node {
	labels := make(map[string]string)
	labels["failure-domain.beta.kubernetes.io/region"] = "region1"
	labels["failure-domain.beta.kubernetes.io/zone"] = "zone1"

	return &corev1.Node{
		Spec:   &corev1.NodeSpec{},
		Status: &corev1.NodeStatus{},
		Metadata: &metav1.ObjectMeta{
			Name:   k8s.String("testNode"),
			Labels: labels,
		},
	}
}

func NewTestK8sClient(url string) *k8s.Client {
	client := &k8s.Client{
		Endpoint:  url,
		Namespace: "",
		Client:    http.DefaultClient,
	}
	return client
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

// Borrowed from github.com/ericchiang/k8s/codec.go
func marshalPB(obj interface{}) ([]byte, error) {
	var magicBytes = []byte{0x6b, 0x38, 0x73, 0x00}
	message, ok := obj.(proto.Message)
	if !ok {
		return nil, fmt.Errorf("expected obj of type proto.Message, got %T", obj)
	}
	payload, err := proto.Marshal(message)
	if err != nil {
		return nil, err
	}

	// The URL path informs the API server what the API group, version, and resource
	// of the object. We don't need to specify it here to talk to the API server.
	body, err := (&runtime.Unknown{Raw: payload}).Marshal()
	if err != nil {
		return nil, err
	}

	d := make([]byte, len(magicBytes)+len(body))
	copy(d[:len(magicBytes)], magicBytes)
	copy(d[len(magicBytes):], body)
	return d, nil
}

func TestGenerateServicesWithProperHealthCheck(t *testing.T) {
	setEnv(t, "testdata/port_definitions_probe_and_service_only.json")
	defer unsetEnv(t)

	pod := testPod()
	path := "/status/ping"
	pod.Spec.Containers[0].LivenessProbe = &corev1.Probe{
		Handler: &corev1.Handler{
			HttpGet: &corev1.HTTPGetAction{
				Path: &path,
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

func TestCheckOneServiceFromWithProperTags(t *testing.T) {
	setEnv(t, "testdata/port_definitions_probe_and_service_only.json")
	defer unsetEnv(t)

	pod := testPod()
	tags := []string{"a", "b", "c"}
	expectedServices := []consul.ServiceInstance{
		{
			Tags: []string{
				"a", "b", "c",
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
			Tags: []string{"a", "b", "c"},
		},
		{
			Tags: []string{"a", "b", "c", "secureConnection:true"},
		},
		{
			Tags: []string{"a", "b", "c", "service-port:31000", "frontend:generic-app", "envoy"},
		},
	}

	services, err := generateServices("serviceName", pod, tags)
	require.NoError(t, err)

	assert.Len(t, services, 3)
	for i, service := range services {
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
