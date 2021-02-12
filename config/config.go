package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	types "github.com/nomad-node-problem-detector/types"
	"github.com/urfave/cli/v2"
)

var ConfigCommand = &cli.Command{
	Name:  "config",
	Usage: "Run config and health checks related commands",
	Subcommands: []*cli.Command{
		{
			Name:  "generate",
			Usage: "Generates the config",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "root-dir",
					Aliases: []string{"d"},
					Usage:   "Location of health checks",
				},
			},
			Action: func(c *cli.Context) error {
				return generateConfig(c)
			},
		},
	},
}

func generateConfig(context *cli.Context) error {
	var err error
	rootDir := context.String("root-dir")
	if rootDir == "" {
		rootDir, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	files, err := ioutil.ReadDir(rootDir)
	if err != nil {
		return err
	}

	var result []types.Config
	directoryExists := false

	for _, fd := range files {
		if !fd.IsDir() || fd.Name() == ".git" {
			continue
		}

		directoryExists = true
		res := types.Config{
			Type: fd.Name(),
		}
		healthCheckDir := filepath.Join(rootDir, fd.Name())
		hcFiles, err := ioutil.ReadDir(healthCheckDir)
		if err != nil {
			return err
		}

		if len(hcFiles) == 0 {
			return fmt.Errorf("Health check missing in %s directory.", healthCheckDir)
		}

		if len(hcFiles) > 1 {
			return fmt.Errorf("There should be only 1 health check present in the %s directory.\nIf health checks are present at a different location, use --root-dir to set the location. npd config generate --help for more details.\n", healthCheckDir)
		}

		res.HealthCheck = hcFiles[0].Name()
		result = append(result, res)
	}

	if !directoryExists {
		return fmt.Errorf("Error in generating config. No health checks present.")
	}

	byteArray, err := json.MarshalIndent(result, "", "\t")
	if err != nil {
		return err
	}

	s := string(byteArray) + "\n"
	configFile := []byte(s)

	configFilePath := filepath.Join(rootDir, "config.json")
	if err := ioutil.WriteFile(configFilePath, configFile, 0644); err != nil {
		return err
	}

	fmt.Printf("Config file: %s generated successfully.\n", configFilePath)
	return nil
}
