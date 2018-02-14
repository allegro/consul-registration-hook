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
	port := int32(8080)
	pod := testPod()
	pod.Metadata.Labels[consulLabelKey] = "serviceName"
	pod.Spec.Containers[0].Name = &containerName
	pod.Spec.Containers[0].Ports = append(pod.Spec.Containers[0].Ports, &corev1.ContainerPort{HostPort: &port})

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

	assert.Equal(t, "__name_8080", service.ID)
	assert.Equal(t, "serviceName", service.Name)
	assert.Equal(t, 8080, service.Port)
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
