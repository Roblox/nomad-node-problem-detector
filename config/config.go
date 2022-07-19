/*
Copyright 2021 Roblox Corporation

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

        http://www.apache.org/licenses/LICENSE-2.0


Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gosuri/uiprogress"
	"github.com/gosuri/uiprogress/util/strutil"
	build "github.com/nomad-node-problem-detector/build"
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
					Usage:   "Location of health checks. Defaults to pwd",
					EnvVars: []string{"ROOT_DIR"},
				},
			},
			Action: func(c *cli.Context) error {
				return generateConfig(c)
			},
		},
		{
			Name:  "build",
			Usage: "Copy your health checks into a docker image",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "image",
					Aliases:  []string{"i"},
					Required: true,
					Usage:    "Fully qualified docker image name",
				},
				&cli.StringFlag{
					Name:    "root-dir",
					Aliases: []string{"d"},
					Usage:   "Location of health checks. Defaults to pwd",
					EnvVars: []string{"ROOT_DIR"},
				},
			},
			Action: func(c *cli.Context) error {
				return buildConfig(c)
			},
		},
	},
}

var steps = []string{
	"creating /var/lib/nnpd",
	"copying health scripts",
	"creating tarball",
	"building docker image",
}

func buildConfig(context *cli.Context) error {
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}

	rootDir := context.String("root-dir")
	if rootDir == "" {
		rootDir = pwd
	}

	image := context.String("image")

	configFilePath := rootDir + "/config.json"
	if _, err := os.Stat(configFilePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("config.json missing in --root-dir: %s. npd config build -h for help", rootDir)
		}
		return err
	}

	var wg sync.WaitGroup
	uiprogress.Start()
	wg.Add(1)
	go func() {
		defer wg.Done()
		bar := uiprogress.AddBar(len(steps)).AppendCompleted().PrependElapsed()
		bar.Width = 50
		bar.PrependFunc(func(b *uiprogress.Bar) string {
			return strutil.Resize("npd: "+steps[b.Current()-1], 30)
		})

		rand.Seed(500)
		for bar.Incr() {
			time.Sleep(time.Millisecond * time.Duration(rand.Intn(2000)))
		}
	}()

	if err = build.BuildImage(image, rootDir); err != nil {
		return err
	}

	wg.Wait()
	fmt.Printf("%s build successfully.\n", image)
	return nil
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
		// Skip files e.g. README.md or hidden directories.
		if !fd.IsDir() || strings.HasPrefix(fd.Name(), ".") {
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
			return fmt.Errorf("health check missing in %s directory", healthCheckDir)
		}

		if len(hcFiles) > 1 {
			return fmt.Errorf("there should be only 1 health check present in the %s directory.\nIf health checks are present at a different location, use --root-dir to set the location. npd config generate --help for more details", healthCheckDir)
		}

		res.HealthCheck = hcFiles[0].Name()
		result = append(result, res)
	}

	if !directoryExists {
		return fmt.Errorf("error in generating config. No health checks present")
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
