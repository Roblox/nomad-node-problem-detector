package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"time"

	aggregator "github.com/nomad-node-problem-detector/aggregator"
	detector "github.com/nomad-node-problem-detector/detector"
	"github.com/urfave/cli/v2"
)

func main() {
	var aggMode bool
	var aggCycleTime string
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
				Destination: &aggMode,
			},
			&cli.StringFlag{
				Name:        "aggregation-cycle-time",
				Value:       "30s",
				Destination: &aggCycleTime,
				Usage:       "Time (in seconds) to wait between each aggregation cycle",
			},
		},
		Action: func(c *cli.Context) error {
			if aggMode {
				aggregator.Aggregate(aggCycleTime)
				return nil
			}

			if c.NArg() > 0 {
				return fmt.Errorf("npd requires 0 arguments.")
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
