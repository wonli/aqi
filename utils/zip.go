package utils

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func Unzip(src, dest string) ([]string, error) {
	r, err := zip.OpenReader(src)
	if err != nil {
		return nil, err
	}

	var unzipFiles []string
	defer func() {
		_ = r.Close()
	}()

	_ = os.MkdirAll(dest, 0755)

	// Closure to address file descriptors issue with all the deferred .Close() methods
	extractAndWriteFile := func(f *zip.File) error {
		rc, err := f.Open()
		if err != nil {
			return err
		}

		defer func() {
			_ = rc.Close()
		}()

		path := filepath.Join(dest, f.Name)
		// Check for ZipSlip (Directory traversal)
		if !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", path)
		}

		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(path, f.Mode())
		} else {
			_ = os.MkdirAll(filepath.Dir(path), f.Mode())
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}

			defer func() {
				_ = f.Close()
			}()

			_, err = io.Copy(f, rc)
			if err != nil {
				return err
			}
		}

		unzipFiles = append(unzipFiles, path)
		return nil
	}

	for _, f := range r.File {
		err = extractAndWriteFile(f)
		if err != nil {
			return nil, err
		}
	}

	return unzipFiles, nil
}
