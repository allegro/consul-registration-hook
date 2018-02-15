package main

import (
	"context"
	"errors"
	"log"
	"os"

	"github.com/urfave/cli"

	"github.com/allegro/consul-registration-hook/consul"
	"github.com/allegro/consul-registration-hook/k8s"
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
					return errors.New("not supported yet")
				},
			},
			{
				Name:  "k8s",
				Usage: "register using data from Kubernetes API",
				Action: func(c *cli.Context) error {
					provider := k8s.ServiceProvider{}
					// TODO(medzin): Add support for timeout here
					services, err := provider.Get(context.Background())
					if err != nil {
						return err
					}
					if err := consul.Register(services); err != nil {
						return err
					}
					return nil
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
				Usage: "register using data from Mesos Agent API",
				Action: func(c *cli.Context) error {
					return errors.New("not supported yet")
				},
			},
			{
				Name:  "k8s",
				Usage: "deregister using data from Kubernetes API",
				Action: func(c *cli.Context) error {
					provider := k8s.ServiceProvider{}
					// TODO(medzin): Add support for timeout here
					services, err := provider.Get(context.Background())
					if err != nil {
						return err
					}
					if err := consul.Deregister(services); err != nil {
						return err
					}
					return nil
				},
			},
		},
	},
}

func main() {
	app := cli.NewApp()
	app.Name = "consul-registration-hook"
	app.Description = "Hook that can be used for synchronous registration and deregistration in Consul discovery service on Kubernetes or Mesos cluster with Allegro executor"
	app.Usage = ""
	app.Commands = commands

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
