package test

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/onsi/ginkgo/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

var (
	DefaultTimeout = 4 * time.Second        //nolint:gochecknoglobals // used in Eventually polls
	DefaultPolling = 250 * time.Millisecond //nolint:gochecknoglobals // used in Eventually polls
)

//go:embed data/expected_authconfig.yaml
var ExpectedAuthConfig []byte

//go:embed fixtures/default_openshift_ingress_config.yaml
var defaultIngressConfig []byte

func DefaultIngressControllerConfig(ctx context.Context, c client.Client) (*unstructured.Unstructured, error) {
	return CreateResource(ctx, c, defaultIngressConfig)
}

func CreateResource(ctx context.Context, cli client.Client, data []byte) (*unstructured.Unstructured, error) {
	unstrObj := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(data, &unstrObj.Object); err != nil {
		return nil, fmt.Errorf("Error unmarshalling YAML to unstructured: %w\n", err)
	}

	return unstrObj, cli.Create(ctx, unstrObj)
}

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
