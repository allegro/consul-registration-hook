package consul

import "time"

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
	Port  int
	Check *Check
}

// Register adds passed service instances to Consul discovery service.
func Register(services []ServiceInstance) error {
	// TODO(medzin): Add consul registration here
	return nil
}

// Deregister removes passed service instances from Consul discovery service.
func Deregister(services []ServiceInstance) error {
	// TODO(medzin): Add consul deregistration here
	return nil
}
