package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/urfave/cli"

	"github.com/allegro/consul-registration-hook/consul"
	"github.com/allegro/consul-registration-hook/hookflags"
	"github.com/allegro/consul-registration-hook/k8s"
	"github.com/allegro/consul-registration-hook/mesos"
)

const (
	flagServiceName   = "service-name"
	envVarServiceName = "KUBERNETES_SERVICE_NAME"

	flagPodIP   = "pod-ip"
	envVarPodIP = "KUBERNETES_POD_IP"

	flagContainerPort   = "container-port"
	envVarContainerPort = "KUBERNETES_CONTAINER_PORT"

	flagServiceTags   = "service-tags"
	envVarServiceTags = "KUBERNETES_SERVICE_TAGS"

	flagCheckPath   = "check-path"
	envVarCheckPath = "KUBERNETES_CHECK_PATH"

	flagServiceID = "service-id"
	envServiceID  = "KUBERNETES_SERVICE_ID"

	flagGetPodTimeout    = "get-pod-timeout"
	envVarGetPodTimeout  = "KUBERNETES_GET_POD_TIMEOUT"
	defaultGetPodTimeout = 10 * time.Second
	consulACLFileFlag    = "consul-acl-file"

	flagHealthCheckTimeout    = "health-check-timeout"
	envVarHealthCheckTimeout  = "KUBERNETES_HEALTH_CHECK_TIMEOUT"
	defaultHealthCheckTimeout = 300 * time.Second
)

var commands = []cli.Command{
	{
		Name: "register",
		Usage: "register service into Consul discovery service.\n\n" +
			"Consul env variables:\n" +
			"- CONSUL_HTTP_ADDR - addr used to register services,\n" +
			"- DISCOVERY_CONSUL_HOST - host used to query for services.\n",
		Subcommands: []cli.Command{
			{
				Name:  "mesos",
				Usage: "register using data from Mesos Agent API",
				Action: func(c *cli.Context) error {
					log.Print("Registering services using data from Mesos API")
					provider := mesos.ServiceProvider{}
					// TODO(medzin): Add support for timeout here
					services, err := provider.Get(context.Background())
					if err != nil {
						return fmt.Errorf("error getting services to register: %s", err)
					}
					log.Printf("Found %d services to register", len(services))
					aclTokenFile := c.Parent().Parent().String(consulACLFileFlag)
					agent := consul.NewAgent(aclTokenFile)
					return agent.Register(services)
				},
			},
			{
				Name:  "k8s",
				Usage: "register using data from Kubernetes API",
				Action: func(c *cli.Context) error {
					log.Print("Registering services using data from Kubernetes API")
					provider := k8s.ServiceProvider{
						Timeout:            c.Duration(flagGetPodTimeout),
						HealthCheckTimeout: c.Duration(flagHealthCheckTimeout),
					}
					// TODO(medzin): Add support for timeout here
					services, err := provider.Get(context.Background())
					if err != nil {
						return fmt.Errorf("error getting services to register: %s", err)
					}
					log.Printf("Found %d services to register", len(services))
					deregisterServices := provider.GenerateSecured(context.Background(), services)
					log.Printf("Found %d services to deregister", len(deregisterServices))
					aclTokenFile := c.Parent().Parent().String(consulACLFileFlag)
					agent := consul.NewAgent(aclTokenFile)
					if len(deregisterServices) > 0 {
						er := agent.Deregister(deregisterServices)
						if er != nil {
							log.Printf("Error deregistering services : %s", er)
						}
					}
					err = provider.CheckProbe(context.Background())
					if err != nil {
						return fmt.Errorf("Error checking services liveness: %s", err)
					}
					return agent.Register(services)
				},
				Flags: []cli.Flag{
					cli.DurationFlag{
						Name:   flagGetPodTimeout,
						Usage:  "change timeout for fetching pod info",
						EnvVar: envVarGetPodTimeout,
						Value:  defaultGetPodTimeout,
					},
					cli.DurationFlag{
						Name:   flagHealthCheckTimeout,
						Usage:  "change consul hook timeout",
						EnvVar: envVarHealthCheckTimeout,
						Value:  defaultHealthCheckTimeout,
					},
				},
			},
			{
				Name:  "cli",
				Usage: "register using data from cli",
				Action: func(c *cli.Context) error {
					log.Print("Registering services using data from cli. Set CONSUL_HTTP_ADDR env to appropriate agent.")
					provider := hookflags.ServiceProvider{
						FlagServiceName:   flagServiceName,
						FlagPodIP:         flagPodIP,
						FlagContainerPort: flagContainerPort,
						FlagServiceTags:   flagServiceTags,
						FlagCheckPath:     flagCheckPath,
						CLIContext:        c,
					}
					services, err := provider.Get(context.Background())
					if err != nil {
						return fmt.Errorf("error getting services to register: %s", err)
					}
					aclTokenFile := c.Parent().Parent().String(consulACLFileFlag)
					agent := consul.NewAgent(aclTokenFile)
					return agent.Register(services)
				},
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:   flagServiceName,
						Usage:  "service name to register by cli",
						EnvVar: envVarServiceName,
					},
					cli.StringFlag{
						Name:   flagPodIP,
						Usage:  "pod ip to register in consul",
						EnvVar: envVarPodIP,
					},
					cli.IntFlag{
						Name:   flagContainerPort,
						Usage:  "container port to register in consul",
						EnvVar: envVarContainerPort,
					},
					cli.StringFlag{
						Name:   flagServiceTags,
						Usage:  "tags to register in consul (comma delimited values: k8sPodNamespace:default,scUid:sc-11298,default-monitoring)",
						EnvVar: envVarServiceTags,
					},
					cli.StringFlag{
						Name:   flagCheckPath,
						Usage:  "health check to register in consul",
						EnvVar: envVarCheckPath,
					},
				},
			},
		},
	},
	{
		Name:  "deregister",
		Usage: "deregister service from Consul discovery service",
		Subcommands: []cli.Command{
			{
				Name:  "mesos",
				Usage: "deregister using data from Mesos Agent API",
				Action: func(c *cli.Context) error {
					log.Print("Deregistering services using data from Mesos API")
					provider := mesos.ServiceProvider{}
					// TODO(medzin): Add support for timeout here
					services, err := provider.Get(context.Background())
					if err != nil {
						return fmt.Errorf("error getting services to deregister: %s", err)
					}
					log.Printf("Found %d services to deregister", len(services))
					aclTokenFile := c.Parent().Parent().String(consulACLFileFlag)
					agent := consul.NewAgent(aclTokenFile)
					return agent.Deregister(services)
				},
			},
			{
				Name:  "k8s",
				Usage: "deregister using data from Kubernetes API",
				Action: func(c *cli.Context) error {
					log.Print("Deregistering services using data from Kubernetes API")
					provider := k8s.ServiceProvider{
						Timeout: c.Duration(flagGetPodTimeout),
					}
					// TODO(medzin): Add support for timeout here
					services, err := provider.Get(context.Background())
					if err != nil {
						return fmt.Errorf("error getting services to deregister: %s", err)
					}
					log.Printf("Found %d services to deregister", len(services))
					aclTokenFile := c.Parent().Parent().String(consulACLFileFlag)
					agent := consul.NewAgent(aclTokenFile)
					return agent.Deregister(services)
				},
				Flags: []cli.Flag{
					cli.DurationFlag{
						Name:   flagGetPodTimeout,
						Usage:  "change timeout for fetching pod info",
						EnvVar: envVarGetPodTimeout,
						Value:  defaultGetPodTimeout,
					},
				},
			},
			{
				Name:  "cli",
				Usage: "Deregister using data from cli. Set CONSUL_HTTP_ADDR env to appropriate agent.",
				Action: func(c *cli.Context) error {
					log.Print("Deregistering services using data from cli")
					services := []consul.ServiceInstance{
						{
							ID: c.String(flagServiceID),
						},
					}

					aclTokenFile := c.Parent().Parent().String(consulACLFileFlag)
					agent := consul.NewAgent(aclTokenFile)
					return agent.Deregister(services)
				},
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:   flagServiceID,
						Usage:  "consul service-id to deregister by cli",
						EnvVar: envServiceID,
					},
				},
			},
		},
	},
}

var version string

func main() {
	app := cli.NewApp()
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  consulACLFileFlag,
			Usage: "Consul acl token file location.",
		},
	}
	app.Name = "consul-registration-hook"
	app.Description = "Hook that can be used for synchronous registration and deregistration in Consul discovery service on Kubernetes or Mesos cluster with Allegro executor"
	app.Usage = ""
	app.Version = version
	app.Commands = commands

	log.Printf("consul-registration-hook (version: %s)", version)
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
