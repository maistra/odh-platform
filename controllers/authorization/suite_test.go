package authorization_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/opendatahub-io/odh-platform/controllers/authorization"
	"github.com/opendatahub-io/odh-platform/pkg/config"
	"github.com/opendatahub-io/odh-platform/pkg/platform"
	"github.com/opendatahub-io/odh-platform/pkg/spi"
	"github.com/opendatahub-io/odh-platform/test"
	"github.com/opendatahub-io/odh-platform/test/k8senvtest"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
)

var envTest *k8senvtest.Client
var cancelFunc context.CancelFunc

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Reconcilers suite")
}

var _ = SynchronizedBeforeSuite(func(ctx context.Context) {
	if !test.IsEnvTest() {
		return
	}

	envTest, cancelFunc = test.StartWithControllers(
		authorization.NewPlatformAuthorizationReconciler(
			nil,
			ctrl.Log.WithName("controllers").WithName("platform"),
			spi.AuthorizationComponent{
				ProtectedResource: platform.ProtectedResource{
					ObjectReference: platform.ObjectReference{
						GroupVersionKind: schema.GroupVersionKind{
							Version: "v1",
							Group:   "opendatahub.io",
							Kind:    "Component",
						},
						Resources: "components",
					},
					WorkloadSelector: map[string]string{},
					Ports:            []string{},
					HostPaths:        []string{"spec.host"},
				},
			},
			authorization.PlatformAuthorizationConfig{
				Label:        config.GetAuthorinoLabel(),
				Audiences:    config.GetAuthAudience(),
				ProviderName: config.GetAuthProvider(),
			},
		).SetupWithManager,
	)

}, func() {})

var _ = SynchronizedAfterSuite(func() {}, func() {
	if !test.IsEnvTest() {
		return
	}

	By("Tearing down the test environment")
	cancelFunc()
	Expect(envTest.Stop()).To(Succeed())
})
