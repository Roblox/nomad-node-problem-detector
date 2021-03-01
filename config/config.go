package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gosuri/uiprogress"
	"github.com/gosuri/uiprogress/util/strutil"
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
				},
			},
			Action: func(c *cli.Context) error {
				return generateConfig(c)
			},
		},
		{
			Name:  "upload",
			Usage: "Upload the config and the health checks into docker registry",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "image",
					Aliases:  []string{"i"},
					Required: true,
					Usage:    "Fully qualified docker image, that will be pushed to the registry",
				},
				&cli.StringFlag{
					Name:    "root-dir",
					Aliases: []string{"d"},
					Usage:   "Location of health checks. Defaults to pwd",
				},
			},
			Action: func(c *cli.Context) error {
				return uploadConfig(c)
			},
		},
	},
}

var steps = []string{
	"creating /var/lib/nnpd",
	"copying health scripts",
	"creating tarball",
	"building docker image",
	"uploading docker image",
}

func uploadConfig(context *cli.Context) error {
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}

	rootDir := context.String("root-dir")
	if rootDir == "" {
		rootDir = pwd
	}

	image := context.String("image")
	pwdSlice := strings.Split(pwd, "/")
	var dockerfilePath string
	if len(pwdSlice) > 1 {
		dockerfilePath = strings.Join(pwdSlice[len(pwdSlice)-2:], "/")
	}

	if dockerfilePath != "nomad-node-problem-detector/build" {
		return fmt.Errorf("npd config upload must be run from project root build directory: nomad-node-problem-detector/build")
	}

	configFilePath := rootDir + "/config.json"
	if _, err := os.Stat(configFilePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("config.json missing in --root-dir: %s. npd config upload -h for help\n", rootDir)
		}
		return err
	}

	uploadCmd := pwd + "/" + "upload.sh"
	cmd := exec.Command(uploadCmd, image, rootDir)

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

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Error in uploading docker image %s: %s: %s\n", image, err.Error(), string(output))
	}

	wg.Wait()
	fmt.Printf("%s build and uploaded successfully.\n", image)
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
