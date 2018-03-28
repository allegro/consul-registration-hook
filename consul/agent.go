package consul

import (
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/consul/api"
	"os"
	"io/ioutil"
	"log"
)

var consulAclTokenEnv = "CONSUL_ACL_TOKEN"
var hostIpEnv = "HOST_IP"

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

type agentClient interface {
	ServiceRegister(*api.AgentServiceRegistration) error
	ServiceDeregister(string) error
}

// Agent is a type responsible for registering and deregistering services in
// Consul agent.
type Agent struct {
	agentClient agentClient
}

// Register adds passed service instances to Consul discovery service.
func (a *Agent) Register(services []ServiceInstance) error {
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

		if err := a.agentClient.ServiceRegister(apiServiceInstance); err != nil {
			return err
		}
	}

	return nil
}

// Deregister removes passed service instances from Consul discovery service.
func (a *Agent) Deregister(services []ServiceInstance) error {
	var errs []error

	for _, service := range services {
		if err := a.agentClient.ServiceDeregister(service.ID); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", errs)
	}

	return nil
}

// NewAgent returns a new Agent.
func NewAgent(tokenFile string) *Agent {
	config := api.DefaultConfig()
	// at this point Token is empty string, if token was not passed on commandline we will just use not secure client
	config.Token = getToken(tokenFile)
	config.Address = getAddress()
	consulClient, _ := api.NewClient(config)
	agent := consulClient.Agent()
	return &Agent{agentClient: agent}
}

func getToken(tokenFile string) string {
	if tokenFile != "" {
		aclBinaryToken, err := ioutil.ReadFile(tokenFile)
		if err != nil {
			log.Print(err)
		}
		return string(aclBinaryToken)
	} else {
		return os.Getenv(consulAclTokenEnv)
	}
}

func getAddress() string {
	hostIp := os.Getenv(hostIpEnv)
	if len(hostIp) == 0 {
		return api.DefaultConfig().Address
	} else {
		return fmt.Sprintf("%s:8500", hostIp)
	}

}
