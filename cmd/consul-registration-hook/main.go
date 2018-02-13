package main

import (
	"log"
	"os"

	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "consul-registration-hook"
	app.Description = "Hook that can be used for synchronous registration and deregistration in Consul discovery service on Kubernetes or Mesos cluster with Allegro executor"
	app.Usage = ""
	app.Commands = []cli.Command{
		{
			Name:  "register",
			Usage: "register service into Consul discovery service",
			Action: func(c *cli.Context) error {
				return nil // TODO(medzin): Write implementation here...
			},
		},
		{
			Name:  "deregister",
			Usage: "deregister service from Consul discovery service",
			Action: func(c *cli.Context) error {
				return nil // TODO(medzin): Write implementation here...
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
