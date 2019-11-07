package hookflags

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/allegro/consul-registration-hook/consul"
	"github.com/urfave/cli"
)

const (
	serviceTagsSeparator = ","
)

// ServiceProvider is responsible for providing services that should be registered
// in Consul discovery service.
type ServiceProvider struct {
	FlagServiceName   string
	FlagPodIP         string
	FlagContainerPort string
	FlagServiceTags   string
	FlagCheckPath     string
	CLIContext        *cli.Context
}

// Get returns slice of services that are configured to be registered in Consul
// discovery service.
func (p *ServiceProvider) Get(ctx context.Context) ([]consul.ServiceInstance, error) {
	serviceName := p.CLIContext.String(p.FlagServiceName)
	host := p.CLIContext.String(p.FlagPodIP)
	port := p.CLIContext.Int(p.FlagContainerPort)
	path := p.CLIContext.String(p.FlagCheckPath)

	service := consul.ServiceInstance{
		ID:    fmt.Sprintf("%s_%d", host, port),
		Name:  serviceName,
		Host:  host,
		Port:  port,
		Check: getConsulHTTPCheck(host, port, path),
	}

	service.Tags = append(service.Tags, p.getTags()...)

	return []consul.ServiceInstance{service}, nil
}

func (p *ServiceProvider) getTags() []string {
	return strings.Split(p.CLIContext.String(p.FlagServiceTags), serviceTagsSeparator)
}

func getConsulHTTPCheck(host string, port int, path string) *consul.Check {
	var checkType consul.CheckType
	var address string

	checkType = consul.CheckHTTPGet
	u := url.URL{
		Host:   fmt.Sprintf("%s:%d", host, port),
		Path:   path,
		Scheme: "http",
	}
	address = u.String()

	interval := 30 * time.Second
	timeout := 30 * time.Second

	return &consul.Check{
		Type:     checkType,
		Address:  address,
		Interval: interval,
		Timeout:  timeout,
	}
}
