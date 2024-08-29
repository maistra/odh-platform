//go:build tools
// +build tools

package tools

// nolint
import (
	_ "github.com/onsi/ginkgo/v2/ginkgo"
	_ "github.com/onsi/ginkgo/v2/ginkgo/generators"
	_ "k8s.io/code-generator"
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
)
