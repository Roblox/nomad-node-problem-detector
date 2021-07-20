package build

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/jhoonb/archivex"
	"github.com/otiai10/copy"
)

func BuildImage(imageName, rootDir string) error {
	// Make temp directory
	tmpDir, err := ioutil.TempDir("", "nnpd-")
	if err != nil {
		return err
	}

	if err = os.MkdirAll(tmpDir+"/var/lib/nnpd", os.ModePerm); err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	// Copy rootDir into temp/var/lib/nnpd
	opt := copy.Options{
		Skip: func(src string) (bool, error) {
			return strings.HasSuffix(src, ".git"), nil
		},
	}
	if err = copy.Copy(rootDir, tmpDir+"/var/lib/nnpd", opt); err != nil {
		return err
	}

	// Change directory to tmpDir
	if err = os.Chdir(tmpDir); err != nil {
		return err
	}

	// Tar temp/var/lib/nnpd
	if err = tarDir(tmpDir+"/nnpd.tar", "var/lib/nnpd"); err != nil {
		return err
	}

	// Write Dockerfile
	d1 := []byte("FROM ubuntu:20.04\nCOPY ./nnpd.tar /tmp/nnpd.tar\nENTRYPOINT tar -xvf /tmp/nnpd.tar -C /alloc >/dev/null 2>&1")
	if err = ioutil.WriteFile(tmpDir+"/Dockerfile", d1, 0744); err != nil {
		return err
	}

	ctx := context.Background()
	cli, err := client.NewEnvClient()
	if err != nil {
		return err
	}

	file, fileInfo, err := openAndStatFile(tmpDir+"/nnpd.tar", os.O_RDWR, os.ModePerm)
	if err != nil {
		return err
	}

	filed, fileInfod, err := openAndStatFile(tmpDir+"/Dockerfile", os.O_RDWR, os.ModePerm)
	if err != nil {
		return nil
	}

	// Add files to build context
	buildContext := new(archivex.TarFile)
	buildContext.Create(tmpDir + "/buildContext.tar")
	buildContext.AddAll(tmpDir+"/.", true)
	buildContext.Add(tmpDir+"/nnpd.tar", file, fileInfo)
	buildContext.Add(tmpDir+"/Dockerfile", filed, fileInfod)
	buildContext.Close()

	dockerBuildContext, err := os.Open(tmpDir + "/buildContext.tar")
	defer dockerBuildContext.Close()

	options := types.ImageBuildOptions{
		SuppressOutput: false,
		Remove:         true,
		ForceRemove:    true,
		PullParent:     true,
		Tags:           []string{imageName},
	}

	imageBuildResponse, err := cli.ImageBuild(
		ctx,
		dockerBuildContext,
		options,
	)
	if err != nil {
		return err
	}
	defer imageBuildResponse.Body.Close()

	if _, err = io.Copy(ioutil.Discard, imageBuildResponse.Body); err != nil {
		return err
	}

	return nil
}

func openAndStatFile(name string, flag int, perm os.FileMode) (*os.File, os.FileInfo, error) {
	file, err := os.OpenFile(name, flag, perm)
	if err != nil {
		return nil, nil, err
	}

	fileInfo, err := os.Stat(name)
	if err != nil {
		return nil, nil, err
	}

	return file, fileInfo, nil
}
