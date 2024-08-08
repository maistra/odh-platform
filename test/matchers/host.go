package matchers

import (
	"errors"

	"github.com/kuadrant/authorino/api/v1beta2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
	openshiftroutev1 "github.com/openshift/api/route/v1"
	"istio.io/client-go/pkg/apis/networking/v1beta1"
)

func HaveHost(name string) types.GomegaMatcher {
	return &hostsMatcher{expectedHosts: []string{name}}
}

func HaveHosts(name ...string) types.GomegaMatcher {
	return &hostsMatcher{expectedHosts: name}
}

type hostsMatcher struct {
	expectedHosts []string
}

func (matcher *hostsMatcher) Match(actual any) (bool, error) {
	if actual == nil {
		return true, nil
	}

	actualHosts, errExtract := extractHosts(actual)
	if errExtract != nil {
		return false, errExtract
	}

	return gomega.ContainElements(matcher.expectedHosts).Match(actualHosts)
}

func (matcher *hostsMatcher) FailureMessage(actual any) string {
	return format.Message(actual, "to have host prefix", matcher.expectedHosts)
}

func (matcher *hostsMatcher) NegatedFailureMessage(actual any) string {
	return format.Message(actual, "to not have host prefix", matcher.expectedHosts)
}

func extractHosts(actual any) ([]string, error) {
	var derefErrors error

	if route, err := deref[openshiftroutev1.Route](actual); err != nil {
		derefErrors = errors.Join(derefErrors, err)
	} else {
		return []string{route.Spec.Host}, nil
	}

	if vs, err := deref[v1beta1.VirtualService](actual); err != nil {
		derefErrors = errors.Join(derefErrors, err)
	} else {
		return vs.Spec.GetHosts(), nil
	}

	if dr, err := deref[v1beta1.DestinationRule](actual); err != nil {
		derefErrors = errors.Join(derefErrors, err)
	} else {
		return []string{dr.Spec.GetHost()}, nil
	}

	if gw, err := deref[v1beta1.Gateway](actual); err != nil {
		derefErrors = errors.Join(derefErrors, err)
	} else {
		return gw.Spec.GetServers()[0].GetHosts(), nil
	}

	if authConfig, err := deref[v1beta2.AuthConfig](actual); err != nil {
		derefErrors = errors.Join(derefErrors, err)
	} else {
		return authConfig.Spec.Hosts, nil
	}

	return []string{""}, derefErrors
}
