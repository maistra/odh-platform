package authorization_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/opendatahub-io/odh-platform/pkg/authorization"
	"github.com/opendatahub-io/odh-platform/test"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = Describe("AuthConfig functions", test.Unit(), func() {

	Context("Host extraction", func() {

		It("should extract host from unstructured via paths as string", func() {
			// given
			extractor := authorization.NewExpressionHostExtractor([]string{"status.url"})
			target := unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"url": "http://test.com",
					},
				},
			}

			// when
			hosts, err := extractor.Extract(&target)

			// then
			Expect(err).To(Not(HaveOccurred()))
			Expect(hosts).To(HaveExactElements("test.com"))
		})

		It("should extract host from unstructured via paths as slice of strings", func() {
			// given
			extractor := authorization.NewExpressionHostExtractor([]string{"status.url"})
			target := unstructured.Unstructured{
				Object: map[string]interface{}{},
			}
			unstructured.SetNestedStringSlice(target.Object, []string{"test.com", "test2.com"}, "status", "url")

			// when
			hosts, err := extractor.Extract(&target)

			// then
			Expect(err).To(Not(HaveOccurred()))
			Expect(hosts).To(ContainElements("test.com", "test2.com"))
		})

	})

})
