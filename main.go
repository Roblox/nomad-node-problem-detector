package main

import (
	log "github.com/sirupsen/logrus"
	"os"
	"time"

	aggregator "github.com/nomad-node-problem-detector/aggregator"
	detector "github.com/nomad-node-problem-detector/detector"
	"github.com/urfave/cli/v2"
)

func main() {
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
		Commands: []*cli.Command{
			aggregator.AggregatorCommand,
			detector.DetectorCommand,
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
