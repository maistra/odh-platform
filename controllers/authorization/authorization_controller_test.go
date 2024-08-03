package authorization_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/opendatahub-io/odh-platform/test"
)

var _ = PDescribe("Service is created", test.EnvTest(), func() {

	It("should work", func() {
		Expect(true).To(BeTrue())
	})

})
