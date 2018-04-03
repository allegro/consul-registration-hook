package k8s

import (
	"context"
	"errors"
	"testing"

	corev1 "github.com/ericchiang/k8s/apis/core/v1"
	metav1 "github.com/ericchiang/k8s/apis/meta/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestIfFailsIfKubernetesAPIFails(t *testing.T) {
	client := &MockClient{}
	client.On("GetPod", context.Background(), "", "").
		Return(nil, errors.New("error")).Once()

	provider := ServiceProvider{
		Client: client,
	}

	services, err := provider.Get(context.Background())

	require.Error(t, err)
	require.Empty(t, services)
}

func TestIfReturnsEmptySliceToPodIsNotLabelledCorrectly(t *testing.T) {
	pod := testPod()

	client := &MockClient{}
	client.On("GetPod", context.Background(), "", "").
		Return(pod, nil).Once()

	provider := ServiceProvider{
		Client: client,
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

	client := &MockClient{}
	client.On("GetPod", context.Background(), "", "").
		Return(pod, nil).Once()

	provider := ServiceProvider{
		Client: client,
	}

	services, err := provider.Get(context.Background())

	require.NoError(t, err)
	require.Len(t, services, 1)
	client.AssertExpectations(t)

	service := services[0]

	assert.Len(t, service.Tags, 0)
	assert.Equal(t, "192.0.2.2_8080", service.ID)
	assert.Equal(t, "serviceName", service.Name)
	assert.Equal(t, 8080, service.Port)
}

func TestIfConvertsLabelsToConsulTags(t *testing.T) {
	containerName := "name"
	port := int32(8080)
	pod := testPod()
	pod.Metadata.Labels["test-tag"] = "tag"
	pod.Metadata.Labels[consulLabelKey] = "serviceName"
	pod.Spec.Containers[0].Name = &containerName
	pod.Spec.Containers[0].Ports = append(pod.Spec.Containers[0].Ports, &corev1.ContainerPort{ContainerPort: &port})

	client := &MockClient{}
	client.On("GetPod", context.Background(), "", "").
		Return(pod, nil).Once()

	provider := ServiceProvider{
		Client: client,
	}

	services, err := provider.Get(context.Background())

	require.NoError(t, err)
	require.Len(t, services, 1)
	client.AssertExpectations(t)

	service := services[0]
	assert.Len(t, service.Tags, 1)
	assert.Contains(t, service.Tags, "test-tag")
}

func testPod() *corev1.Pod {
	return &corev1.Pod{
		Spec: &corev1.PodSpec{
			Containers: []*corev1.Container{
				{
					Ports: []*corev1.ContainerPort{},
				},
			},
		},
		Status: &corev1.PodStatus{},
		Metadata: &metav1.ObjectMeta{
			Labels: make(map[string]string),
		},
	}
}

type MockClient struct {
	mock.Mock
}

func (c *MockClient) GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error) {
	args := c.Called(ctx, namespace, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*corev1.Pod), args.Error(1)
}
