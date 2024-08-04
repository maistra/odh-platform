package test

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/onsi/ginkgo/v2"
)

//go:embed data/expected_authconfig.yaml
var ExpectedAuthConfig []byte

func ProjectRoot() string {
	rootDir := ""

	currentDir, err := os.Getwd()
	if err != nil {
		ginkgo.Fail(fmt.Sprintf("failed to get current working directory: %v", err))
	}

	for {
		if _, err := os.Stat(filepath.Join(currentDir, "go.mod")); err == nil {
			rootDir = filepath.FromSlash(currentDir)

			break
		}

		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			break
		}

		currentDir = parentDir
	}

	if rootDir == "" {
		ginkgo.Fail(fmt.Sprintf("failed to get current working directory: %v", err))
	}

	return rootDir
}
