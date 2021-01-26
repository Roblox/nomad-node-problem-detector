package main

import (
	"log"
	"os"

	"github.com/urfave/cli/v2"
	"time"
)

func main() {
	var aggregator bool
	app := &cli.App{
		Name:                 "npd",
		Usage:                "Nomad node problem detector",
		Version:              "v1.0.0",
		Compiled:             time.Now(),
		EnableBashCompletion: true,
		Authors: []*cli.Author{
			{
				Name:  "Shishir Mahajan",
				Email: "smahajan@roblox.com",
			},
		},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "aggregator",
				Usage:       "Run npd in aggregator mode",
				Destination: &aggregator,
			},
		},
		Action: func(c *cli.Context) error {
			if aggregator {
				aggregate()
				return nil
			}

			detectNodeProblems()
			return nil
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}

}
