package config_test

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/opendatahub-io/odh-platform/pkg/config"
	"github.com/opendatahub-io/odh-platform/pkg/platform"
	"github.com/opendatahub-io/odh-platform/test"
)

var _ = Describe("Loading capabilities", test.Unit(), func() {

	Context("loading capabilities from files", func() {

		It("should load authorized resources", func() {
			configPath := filepath.Join(test.ProjectRoot(), "test", "data", "config", "authorization")

			var protectedResources []platform.ProtectedResource
			Expect(config.Load(&protectedResources, configPath)).To(Succeed())
			Expect(protectedResources).To(
				ContainElement(
					MatchFields(IgnoreExtras, Fields{
						"Ports":     ContainElement("9192"),
						"HostPaths": ContainElement("status.url"),
					}),
				),
			)
		})

	})
})
