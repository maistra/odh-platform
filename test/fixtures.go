package test

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/onsi/ginkgo/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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

func CreateOrUpdateResource(ctx context.Context, cli client.Client, data []byte) (*unstructured.Unstructured, error) {
	unstrObj := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(data, &unstrObj.Object); err != nil {
		return nil, fmt.Errorf("error unmarshalling YAML to unstructured: %w", err)
	}

	_, err := controllerutil.CreateOrUpdate(ctx, cli, unstrObj, func() error {
		return nil
	})

	return unstrObj, err
}
