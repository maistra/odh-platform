package config_test

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/opendatahub-io/odh-platform/pkg/config"
	"github.com/opendatahub-io/odh-platform/pkg/spi"
	"github.com/opendatahub-io/odh-platform/test"
)

var _ = Describe("Loading capabilities", test.Unit(), func() {

	Context("loading capabilities from files", func() {

		It("should load authorized resources", func() {
			configPath := filepath.Join(test.ProjectRoot(), "test", "data", "config")

			authorizationComponents, err := config.Load(spi.AuthorizationComponent{}, configPath)
			Expect(err).To(Succeed())
			Expect(authorizationComponents).To(ContainElement(MatchFields(IgnoreExtras, Fields{
				"ProtectedResource": MatchFields(IgnoreExtras, Fields{
					"Ports":     ContainElement("9192"),
					"HostPaths": ContainElement("status.url"),
				}),
			})))
		})
	})

})
