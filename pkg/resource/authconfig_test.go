package resource_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/opendatahub-io/odh-platform/pkg/resource"
	"github.com/opendatahub-io/odh-platform/test/labels"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = Describe("AuthConfig functions", Label(labels.Unit), func() {

	Context("Host extraction", func() {

		It("should extract host from unstructured via paths as string", func() {
			// given
			extractor := resource.NewExpressionHostExtractor([]string{"status.url"})
			target := unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"url": "http://test.com",
					},
				},
			}

			// when
			hosts := extractor.Extract(&target)

			// then
			Expect(hosts).To(HaveExactElements("test.com"))
		})

		It("should extract host from unstructured via paths as slice of strings", func() {
			// given
			extractor := resource.NewExpressionHostExtractor([]string{"status.url"})
			target := unstructured.Unstructured{
				Object: map[string]interface{}{},
			}
			unstructured.SetNestedStringSlice(target.Object, []string{"test.com", "test2.com"}, "status", "url")

			// when
			hosts := extractor.Extract(&target)

			// then
			Expect(hosts).To(ContainElements("test.com", "test2.com"))
		})

	})

})
