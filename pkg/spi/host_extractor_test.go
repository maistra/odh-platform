package spi_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/opendatahub-io/odh-platform/pkg/spi"
	"github.com/opendatahub-io/odh-platform/test"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = Describe("Host extraction", test.Unit(), func() {

	It("should extract host from unstructured via paths as string", func() {
		// given
		extractor := spi.NewPathExpressionExtractor([]string{"status.url"})
		target := unstructured.Unstructured{
			Object: map[string]interface{}{
				"status": map[string]interface{}{
					"url": "http://test.com",
				},
			},
		}

		// when
		hosts, err := extractor(&target)

		// then
		Expect(err).To(Not(HaveOccurred()))
		Expect(hosts).To(HaveExactElements("http://test.com"))
	})

	It("should extract host from unstructured via paths as slice of strings", func() {
		// given
		extractor := spi.NewPathExpressionExtractor([]string{"status.url"})
		target := unstructured.Unstructured{
			Object: map[string]interface{}{},
		}
		Expect(unstructured.SetNestedStringSlice(target.Object, []string{"test.com", "test2.com"}, "status", "url")).To(Succeed())

		// when
		hosts, err := extractor(&target)

		// then
		Expect(err).To(Not(HaveOccurred()))
		Expect(hosts).To(ContainElements("test.com", "test2.com"))
	})

	It("should return unique list", func() {

		// given
		extractor := spi.NewPathExpressionExtractor([]string{"status.url"})
		target := unstructured.Unstructured{
			Object: map[string]interface{}{},
		}
		Expect(unstructured.SetNestedStringSlice(target.Object, []string{"test.com", "http://test.com", "https://test.com"}, "status", "url")).To(Succeed())

		// when
		hosts, err := spi.UnifiedHostExtractor(extractor)(&target)
		// then
		Expect(err).To(Not(HaveOccurred()))
		Expect(hosts).To(HaveExactElements("test.com"))

	})

})
