package consul

import (
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestIfRegistersServiceInConsul(t *testing.T) {
	service := ServiceInstance{
		ID:   "id",
		Name: "serviceName",
		Host: "myhost",
		Port: 1234,
		Check: &Check{
			Type:     CheckTCP,
			Address:  "localhost:1234",
			Interval: time.Second,
			Timeout:  time.Second,
		},
	}

	mockAgentClient := &MockAgentClient{}
	mockAgentClient.On("ServiceRegister", mock.MatchedBy(func(registration *api.AgentServiceRegistration) bool {
		return registration.ID == service.ID &&
			registration.Name == service.Name &&
			registration.Address == service.Host &&
			registration.Port == service.Port

	})).Return(nil).Once()

	agent := Agent{agentClient: mockAgentClient}

	err := agent.Register([]ServiceInstance{service})

	require.NoError(t, err)
	mockAgentClient.AssertExpectations(t)
}

func TestIfDeregistersServicesInConsul(t *testing.T) {
	services := []ServiceInstance{
		{ID: "id1"},
		{ID: "id2"},
	}

	mockAgentClient := &MockAgentClient{}
	mockAgentClient.On("ServiceDeregister", "id1").Return(nil).Once()
	mockAgentClient.On("ServiceDeregister", "id2").Return(nil).Once()

	agent := Agent{agentClient: mockAgentClient}

	err := agent.Deregister(services)

	require.NoError(t, err)
	mockAgentClient.AssertExpectations(t)
}

func TestIfTriesToDeregisterRegardlessOfErrors(t *testing.T) {
	services := []ServiceInstance{
		{ID: "id1"},
		{ID: "id2"},
	}

	mockAgentClient := &MockAgentClient{}
	mockAgentClient.On("ServiceDeregister", "id1").Return(errors.New("error")).Once()
	mockAgentClient.On("ServiceDeregister", "id2").Return(errors.New("error")).Once()

	agent := Agent{agentClient: mockAgentClient}

	err := agent.Deregister(services)

	require.Error(t, err)
	mockAgentClient.AssertExpectations(t)
}

type MockAgentClient struct {
	mock.Mock
}

func (m *MockAgentClient) ServiceRegister(agentServiceRegistration *api.AgentServiceRegistration) error {
	args := m.Called(agentServiceRegistration)
	return args.Error(0)
}

func (m *MockAgentClient) ServiceDeregister(serviceID string) error {
	args := m.Called(serviceID)
	return args.Error(0)
}
