package mesos

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/wix-playground/consul-registration-hook/consul"
)

const consulLabelKey = "consul"

// ServiceProvider is responsible for providing services that should be registered
// in Consul discovery service.
type ServiceProvider struct {
	agentClient agentClient
}

// Get returns slice of services that are configured to be registered in Consul
// discovery service.
func (p *ServiceProvider) Get(ctx context.Context) ([]consul.ServiceInstance, error) {
	agentClient := p.client()
	state, err := agentClient.state()
	if err != nil {
		return nil, fmt.Errorf("agent api error: %s", err)
	}

	task, err := p.getTaskFromState(state)
	if err != nil {
		return nil, fmt.Errorf("unable to find task info: %s", err)
	}

	return p.buildServices(task)
}

func (p *ServiceProvider) buildServices(t task) ([]consul.ServiceInstance, error) {
	hostname, err := p.getMesosHostname()
	if err != nil {
		return nil, fmt.Errorf("unable to determine hostname: %s", err)
	}

	var services []consul.ServiceInstance
	var tags []string

	for _, label := range t.Labels {
		if label.Value == "tag" {
			tags = append(tags, label.Key)
		}
	}

	// TODO(medzin): add check conversion after MESOS-8780 is completed
	// See: https://issues.apache.org/jira/browse/MESOS-8780
	for _, port := range t.Discovery.Ports.Ports {
		if consulServiceName := p.getConsulServiceName(port.Labels); consulServiceName != "" {
			service := consul.ServiceInstance{
				ID:   fmt.Sprintf("%s_%d", hostname, port.Number),
				Name: consulServiceName,
				Host: hostname,
				Port: port.Number,
				Tags: tags,
			}
			services = append(services, service)
		}
	}

	if len(services) == 0 && len(t.Discovery.Ports.Ports) > 0 {
		if consulServiceName := p.getConsulServiceName(t.Labels); consulServiceName != "" {
			port := t.Discovery.Ports.Ports[0].Number
			service := consul.ServiceInstance{
				ID:   fmt.Sprintf("%s_%d", hostname, port),
				Name: consulServiceName,
				Host: hostname,
				Port: port,
				Tags: tags,
			}
			services = append(services, service)
		}
	}

	return services, nil
}

func (p *ServiceProvider) client() agentClient {
	if p.agentClient != nil {
		return p.agentClient
	}
	return defaultAgentClient{baseURL: defaultAgentBaseURL}
}

func (p *ServiceProvider) getConsulServiceName(labels []label) string {
	for _, label := range labels {
		if label.Key == consulLabelKey {
			return label.Value
		}
	}
	return ""
}

func (p *ServiceProvider) getExecutorAndFrameworkID() (executorID, frameworkID string, err error) {
	if executorID = os.Getenv("MESOS_EXECUTOR_ID"); len(executorID) == 0 {
		err = errors.New("missing MESOS_EXECUTOR_ID environmental variable")
	} else if frameworkID = os.Getenv("MESOS_FRAMEWORK_ID"); len(frameworkID) == 0 {
		err = errors.New("missing MESOS_FRAMEWORK_ID environmental variable")
	}
	return executorID, frameworkID, err
}

func (p *ServiceProvider) getMesosHostname() (string, error) {
	hostname := os.Getenv("HOST")
	if hostname == "" {
		return "", errors.New("missing HOST environmental variable")
	}
	return hostname, nil
}

func (p *ServiceProvider) getTaskFromState(state state) (task, error) {
	executorID, frameworkID, err := p.getExecutorAndFrameworkID()
	if err != nil {
		return task{}, fmt.Errorf("not enough data to search for task info: %s", err)
	}

	for _, framework := range state.Frameworks {
		if framework.ID == frameworkID {
			for _, executor := range framework.Executors {
				if executor.ID == executorID {
					if len(executor.Tasks) > 0 {
						return executor.Tasks[0], nil
					}
				}
			}
		}
	}

	return task{}, errors.New("no task in executor")
}
