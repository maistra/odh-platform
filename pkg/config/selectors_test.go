package config_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/opendatahub-io/odh-platform/pkg/config"
	"github.com/opendatahub-io/odh-platform/test"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = Describe("Templated selectors", test.Unit(), func() {

	Context("simple expressions", func() {

		It("should resolve simple expressions for both key and value", func() {
			labels := map[string]string{
				"A.{{.kind}}": "{{.metadata.name}}",
				"B":           "{{.kind}}",
			}

			target := unstructured.Unstructured{
				Object: map[string]any{},
			}
			target.SetName("X")
			target.SetKind("Y")

			renderedLabels, err := config.ResolveSelectors(labels, &target)
			Expect(err).ToNot(HaveOccurred())

			Expect(renderedLabels["A.Y"]).To(Equal("X"))
			Expect(renderedLabels["B"]).To(Equal("Y"))
		})

		It("should fail on missing expression", func() {
			labels := map[string]string{
				"A": "{{.metadata.name}}",
			}

			target := unstructured.Unstructured{
				Object: map[string]any{},
			}
			target.SetKind("Y")

			_, err := config.ResolveSelectors(labels, &target)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("could not execute template"))
			Expect(err.Error()).To(ContainSubstring("could not resolve value"))
		})
	})

})
