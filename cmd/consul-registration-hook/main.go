package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/urfave/cli"

	"github.com/wix-playground/consul-registration-hook/consul"
	"github.com/wix-playground/consul-registration-hook/k8s"
	"github.com/wix-playground/consul-registration-hook/mesos"
)

const (
	flagGetPodTimeout    = "get-pod-timeout"
	envVarGetPodTimeout  = "KUBERNETES_GET_POD_TIMEOUT"
	defaultGetPodTimeout = 10 * time.Second
	consulACLFileFlag    = "consul-acl-file"
)

var commands = []cli.Command{
	{
		Name:  "register",
		Usage: "register service into Consul discovery service",
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
						Timeout: defaultGetPodTimeout,
					}
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
				Flags: []cli.Flag{
					cli.DurationFlag{
						Name:   flagGetPodTimeout,
						Usage:  "change timeout for fetching pod info",
						EnvVar: envVarGetPodTimeout,
						Value:  defaultGetPodTimeout,
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
						Timeout: defaultGetPodTimeout,
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
