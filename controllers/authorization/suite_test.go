package authorization_test

import (
	"context"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/opendatahub-io/odh-platform/controllers/authorization"
	"github.com/opendatahub-io/odh-platform/pkg/config"
	pschema "github.com/opendatahub-io/odh-platform/pkg/schema"
	"github.com/opendatahub-io/odh-platform/pkg/spi"
	"github.com/opendatahub-io/odh-platform/test"
	"github.com/opendatahub-io/odh-platform/test/k8senvtest"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

var envTest *k8senvtest.Client

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Reconcilers suite")
}

var _ = SynchronizedBeforeSuite(func(ctx context.Context) {
	if !test.IsEnvTest() {
		return
	}

	testScheme := runtime.NewScheme()
	pschema.RegisterSchemes(testScheme)
	utilruntime.Must(v1.AddToScheme(testScheme))

	authzCtrl := authorization.NewPlatformAuthorizationReconciler(
		nil, // SetupWithManager will ensure the client defined for the manager is propagated to the controller under test
		ctrl.Log.WithName("controllers").WithName("platform"),
		spi.AuthorizationComponent{
			CustomResourceType: spi.ResourceSchema{
				GroupVersionKind: schema.GroupVersionKind{Version: "v1", Kind: "service"},
				Resources:        "services",
			},
			WorkloadSelector: map[string]string{},
			Ports:            []string{},
			HostPaths:        []string{"status.url"},
		},
		authorization.PlatformAuthorizationConfig{
			Label:        config.GetAuthorinoLabel(),
			Audiences:    config.GetAuthAudience(),
			ProviderName: config.GetAuthProvider(),
		},
	)

	envTest = k8senvtest.Configure(
		k8senvtest.WithCRDs(filepath.Join(test.ProjectRoot(), "config", "crd", "external")),
		k8senvtest.WithScheme(testScheme),
	).
		WithControllers(authzCtrl.SetupWithManager).
		Start(ctx)

}, func() {})

var _ = SynchronizedAfterSuite(func() {}, func() {
	if !test.IsEnvTest() {
		return
	}
	By("Tearing down the test environment")
	Expect(envTest.Stop()).To(Succeed())
})
