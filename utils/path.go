package utils

import (
	"os"
	"strings"
	"time"
)

func GetFilenamePath(prefix string) string {
	//gen ymd path
	p := time.Now().Format("/2006/01/02/")
	if prefix == "" {
		return p
	}

	return strings.TrimRight(prefix, "/") + p
}

// PathExists check path
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}

	if os.IsNotExist(err) {
		return false, nil
	}

	return false, err
}
