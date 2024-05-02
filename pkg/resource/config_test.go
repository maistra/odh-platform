package resource_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/opendatahub-io/odh-platform/pkg/resource"
	"github.com/opendatahub-io/odh-platform/test/labels"
)

var _ = Describe("Config functions", Label(labels.Unit), func() {

	FWhen("Load", func() {

		It("should find all files", func() {

			components, err := resource.LoadConfig("../../test/data/config")

			Expect(err).To(Succeed())
			Expect(components).To(HaveLen(2))
		})

	})

})
