package routing_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/opendatahub-io/odh-platform/controllers/routing"
	"github.com/opendatahub-io/odh-platform/pkg/platform"
	"github.com/opendatahub-io/odh-platform/pkg/spi"
	"github.com/opendatahub-io/odh-platform/test"
	"github.com/opendatahub-io/odh-platform/test/k8senvtest"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	envTest              *k8senvtest.Client
	routingConfiguration = spi.PlatformRoutingConfiguration{
		IngressService:       "odh-router",
		GatewayNamespace:     "odh-gateway",
		IngressSelectorLabel: "istio",
		IngressSelectorValue: "opendatahub-ingress-gateway",
	}

	cancel context.CancelFunc
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Routing reconciliation")
}

var _ = SynchronizedBeforeSuite(func() {
	if !test.IsEnvTest() {
		return
	}

	routingCtrl := routing.NewPlatformRoutingReconciler(
		nil,
		ctrl.Log.WithName("controllers").WithName("platform"),
		spi.RoutingComponent{
			RoutingTarget: platform.RoutingTarget{
				ObjectReference: platform.ObjectReference{
					GroupVersionKind: schema.GroupVersionKind{
						Version: "v1",
						Group:   "opendatahub.io",
						Kind:    "Component",
					},
				},
			},
		},
		routingConfiguration,
	)

	envTest, cancel = test.StartWithControllers(routingCtrl.SetupWithManager)
}, func() {})

var _ = SynchronizedAfterSuite(func() {}, func() {
	if !test.IsEnvTest() {
		return
	}
	By("Tearing down the test environment")
	cancel()
	Expect(envTest.Stop()).To(Succeed())
})
