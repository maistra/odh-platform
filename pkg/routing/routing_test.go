package routing_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/opendatahub-io/odh-platform/pkg/routing"
	"github.com/opendatahub-io/odh-platform/pkg/spi"
	"github.com/opendatahub-io/odh-platform/test"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
)

var _ = Describe("Resource functions", test.Unit(), func() {

	Context("Template Loader", func() {

		config := routing.IngressConfig{
			GatewayNamespace:     "opendatahub",
			IngressSelectorLabel: "istio",
			IngressSelectorValue: "rhoai-gateway",
			IngressService:       "rhoai-router-ingress",
		}
		httpPort := corev1.ServicePort{
			Name:        "http-api",
			Port:        80,
			AppProtocol: ptr.To("http"),
		}
		grpcPort := corev1.ServicePort{
			Name:        "grpc-api",
			Port:        90,
			AppProtocol: ptr.To("grpc"),
		}

		data := routing.NewExposedServiceConfig(&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "registry",
				Namespace: "office",
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					httpPort,
					grpcPort,
				},
			},
		},
			httpPort, config, "app-crc.testing")

		It("should load public resources", func() {
			// given
			// data^

			// when
			res, err := routing.NewStaticTemplateLoader().Load(data, routing.PublicRoute)

			// then
			Expect(err).ShouldNot(HaveOccurred())
			Expect(res).To(HaveLen(4))
		})

		It("should load external resources", func() {
			// given
			// data^

			// when
			res, err := routing.NewStaticTemplateLoader().Load(data, routing.ExternalRoute)

			// then
			Expect(err).ShouldNot(HaveOccurred())
			Expect(res).To(HaveLen(2))
		})

	})

	Context("Host extraction", func() {

		It("should extract host from unstructured via paths as string", func() {
			// given
			extractor := spi.NewAnnotationHostExtractor(";", "A", "B")
			target := unstructured.Unstructured{
				Object: map[string]any{},
			}
			target.SetAnnotations(map[string]string{
				"A": "a.com;a2.com",
				"B": "b.com;b2.com",
			})

			// when
			hosts, err := extractor(&target)

			// then
			Expect(err).To(Not(HaveOccurred()))
			Expect(hosts).To(HaveExactElements("a.com", "a2.com", "b.com", "b2.com"))
		})

	})

})
