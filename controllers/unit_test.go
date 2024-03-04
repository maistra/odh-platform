package controllers_test

import (
	. "github.com/onsi/ginkgo/v2"
	//. "github.com/onsi/gomega"
	"github.com/opendatahub-io/odh-platform/test/labels"
)

var _ = Describe("Controller helper functions", Label(labels.Unit), func() {

	When("Preparing host URL for Authorino's AuthConfig", func() {

		DescribeTable("it should remove protocol prefix from provided string and path")

	})

	When("Checking namespace", func() {

		DescribeTable("it should not process reserved namespaces")

	})

})
