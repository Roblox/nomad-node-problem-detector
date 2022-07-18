package build

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
)

func tarDir(destinationFilename, sourceDir string) error {
	if destinationFilename[len(destinationFilename)-3:] != "tar" {
		return fmt.Errorf("please provide a valid tar filename")
	}

	tarFile, err := os.Create(destinationFilename)
	if err != nil {
		return err
	}

	defer tarFile.Close()

	var fileWriter io.WriteCloser = tarFile
	tarfileWriter := tar.NewWriter(fileWriter)
	defer tarfileWriter.Close()

	if err = walkTar(sourceDir, tarfileWriter); err != nil {
		return err
	}

	return nil
}

func walkTar(dirPath string, tarfileWriter *tar.Writer) error {
	dir, err := os.Open(dirPath)
	if err != nil {
		return err
	}

	dirInfo, err := dir.Stat()
	if err != nil {
		return err
	}

	// prepare the tar header for dir entry.
	dheader, err := tar.FileInfoHeader(dirInfo, "")
	if err != nil {
		return err
	}

	dheader.Name = dir.Name()

	if err = tarfileWriter.WriteHeader(dheader); err != nil {
		return err
	}

	files, err := dir.Readdir(0) // grab the files list
	if err != nil {
		return err
	}

	for _, fileInfo := range files {
		if fileInfo.IsDir() {
			if err = walkTar(path.Join(dir.Name(), fileInfo.Name()), tarfileWriter); err != nil {
				return err
			}

		} else {
			file, err := os.Open(dir.Name() + string(filepath.Separator) + fileInfo.Name())
			if err != nil {
				return err
			}

			defer file.Close()

			// prepare the tar header for file entry.

			header, err := tar.FileInfoHeader(fileInfo, "")
			if err != nil {
				return err
			}

			header.Name = file.Name()

			if err = tarfileWriter.WriteHeader(header); err != nil {
				return err
			}

			_, err = io.Copy(tarfileWriter, file)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
