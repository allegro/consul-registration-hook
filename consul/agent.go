package consul

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/hashicorp/consul/api"
)

var consulACLTokenEnv = "CONSUL_ACL_TOKEN"
var hostIPEnv = "HOST_IP"

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
	config.Token = getAgentToken(tokenFile)
	config.Address = getAgentAddress()
	consulClient, _ := api.NewClient(config)
	agent := consulClient.Agent()
	return &Agent{agentClient: agent}
}

func getAgentToken(tokenFile string) string {
	if !isEmpty(tokenFile) {
		aclBinaryToken, err := ioutil.ReadFile(tokenFile)
		if err != nil {
			log.Print(err)
		}
		return string(aclBinaryToken)
	}
	return os.Getenv(consulACLTokenEnv)
}

// getAgentAddress is used to determine where consul agent is available,
// currently we expect consul to be available as a DaemonSet on local node.
func getAgentAddress() string {
	hostIP := os.Getenv(hostIPEnv)
	// TODO(tz) - if empty try to fetch it from internal api
	if isEmpty(hostIP) {
		return api.DefaultConfig().Address
	}
	return fmt.Sprintf("%s:8500", hostIP)
}

func isEmpty(input string) bool {
	return len(input) == 0
}
