package test

import (
	"context"
	"path/filepath"

	"github.com/opendatahub-io/odh-platform/controllers"
	pschema "github.com/opendatahub-io/odh-platform/pkg/schema"
	"github.com/opendatahub-io/odh-platform/test/k8senvtest"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

func StartWithControllers(ctrls ...controllers.SetupWithManagerFunc) (*k8senvtest.Client, context.CancelFunc) {
	// The context passed to Process 1, which is invoked before all parallel nodes are started by Ginkgo,
	// is terminated when this function exits. As a result, this context is unsuitable for use with
	// manager/controllers that need to be available for the entire duration of the test suite.
	// To address this, a new cancellable context must be created to ensure it remains active
	// throughout the test suite.
	ctx, cancel := context.WithCancel(context.TODO())

	testScheme := runtime.NewScheme()
	pschema.RegisterSchemes(testScheme)
	utilruntime.Must(apiextv1.AddToScheme(testScheme))

	return k8senvtest.Configure(
		k8senvtest.WithCRDs(
			filepath.Join(ProjectRoot(), "config", "crd", "external"),
			filepath.Join(ProjectRoot(), "test", "data", "crds"),
		),
		k8senvtest.WithScheme(testScheme),
	).WithControllers(ctrls...).
		Start(ctx), cancel
}
