package utils

import (
	"os"
	"path/filepath"
)

func CreateFileIfNotExists(filePath string) error {
	// Check if the file exists
	_, err := os.Stat(filePath)
	if err == nil {
		// File exists
		return nil
	}

	// Create the directory
	dir := filepath.Dir(filePath)
	err = os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		// Failed to create directory
		return err
	}

	// Create an empty file
	f, err := os.Create(filePath)
	if err != nil {
		// Failed to create file
		return err
	}

	defer f.Close()
	return nil
}
