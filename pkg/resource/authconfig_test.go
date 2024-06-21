package resource_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/opendatahub-io/odh-platform/pkg/resource"
	"github.com/opendatahub-io/odh-platform/test/labels"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = Describe("Authconfig functions", Label(labels.Unit), func() {

	When("Host extractor", func() {

		It("should extract host from unstrucured via paths", func() {

			extractor := resource.NewExpressionHostExtractor([]string{"status.url"})
			target := unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"url": "http://test.com",
					},
				},
			}

			Expect(extractor.Extract(&target)).To(Equal([]string{"test.com"}))
		})

	})

})
