package test

import "github.com/onsi/ginkgo/v2"

func Unit() ginkgo.Labels {
	return ginkgo.Label("unit")
}

func EnvTest() ginkgo.Labels {
	return ginkgo.Label("kube-envtest")
}

func IsEnvTest() bool {
	return EnvTest().MatchesLabelFilter(ginkgo.GinkgoLabelFilter())
}
