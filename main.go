package main

import (
	"log"
	"os"
	"time"

	aggregator "github.com/nomad-node-problem-detector/aggregator"
	detector "github.com/nomad-node-problem-detector/detector"
	"github.com/urfave/cli/v2"
)

func main() {
	var aggregatorMode bool
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
				Destination: &aggregatorMode,
			},
		},
		Action: func(c *cli.Context) error {
			if aggregatorMode {
				aggregator.Aggregate()
				return nil
			}

			detector.StartNpdHttpServer()
			return nil
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}

}
