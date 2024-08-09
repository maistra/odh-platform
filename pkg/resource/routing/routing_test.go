package routing_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/opendatahub-io/odh-platform/pkg/resource/routing"
	"github.com/opendatahub-io/odh-platform/pkg/spi"
	"github.com/opendatahub-io/odh-platform/test"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Resource functions", test.Unit(), func() {

	Context("Template Loader", func() {

		data := spi.RoutingTemplateData{
			PlatformRoutingConfiguration: spi.PlatformRoutingConfiguration{
				GatewayNamespace:     "opendatahub",
				IngressSelectorLabel: "istio",
				IngressSelectorValue: "rhoai-gateway",
				IngressService:       "rhoai-router-ingress",
			},
			PublicServiceName: "registry-office",
			ServiceName:       "registry",
			ServiceNamespace:  "office",
			Domain:            "app-crc.testing",
		}

		It("should load public resources", func() {
			// given
			// data^

			// when
			res, err := routing.NewStaticTemplateLoader().Load(context.Background(), spi.PublicRoute, types.NamespacedName{}, data)

			// then
			Expect(err).ShouldNot(HaveOccurred())
			Expect(res).To(HaveLen(4))
		})

		It("should load external resources", func() {
			// given
			// data^

			// when
			res, err := routing.NewStaticTemplateLoader().Load(context.Background(), spi.ExternalRoute, types.NamespacedName{}, data)

			// then
			Expect(err).ShouldNot(HaveOccurred())
			Expect(res).To(HaveLen(2))
		})

	})

})
