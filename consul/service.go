package consul

import (
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/consul/api"
)

// CheckType is a health check type.
type CheckType string

const (
	// CheckHTTPGet represents HTTP GET CheckType.
	CheckHTTPGet = CheckType("HTTP_GET")
	// CheckTCP represents TCP CheckType.
	CheckTCP = CheckType("TCP")
)

// Check represents a Consul health check definition.
type Check struct {
	Type     CheckType
	Address  string
	Interval time.Duration
	Timeout  time.Duration
}

// ServiceInstance represents a Consul service that should be registered.
type ServiceInstance struct {
	ID    string
	Name  string
	Host  string
	Port  int
	Check *Check
}

// Register adds passed service instances to Consul discovery service.
func Register(services []ServiceInstance) error {
	consulClient, _ := api.NewClient(api.DefaultConfig())
	agent := consulClient.Agent()

	for _, service := range services {
		var check *api.AgentServiceCheck
		if service.Check != nil {
			check = &api.AgentServiceCheck{
				Interval: service.Check.Interval.String(),
				Timeout:  service.Check.Timeout.String(),
			}

			switch service.Check.Type {
			case CheckHTTPGet:
				check.HTTP = service.Check.Address
				check.Method = http.MethodGet
			case CheckTCP:
				check.TCP = service.Check.Address
			}
		}

		apiServiceInstance := &api.AgentServiceRegistration{
			ID:      service.ID,
			Name:    service.Name,
			Port:    service.Port,
			Address: service.Host,
			Check:   check,
		}

		if err := agent.ServiceRegister(apiServiceInstance); err != nil {
			return err
		}
	}

	return nil
}

// Deregister removes passed service instances from Consul discovery service.
func Deregister(services []ServiceInstance) error {
	consulClient, _ := api.NewClient(api.DefaultConfig())
	agent := consulClient.Agent()

	var errs []error

	for _, service := range services {
		if err := agent.ServiceDeregister(service.ID); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", errs)
	}

	return nil
}
