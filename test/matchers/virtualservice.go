package matchers

import (
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	"istio.io/client-go/pkg/apis/networking/v1beta1"
)

// BeAttachedToGateways ensures that the VirtualService is attached to only specified gateways.
func BeAttachedToGateways(targetGw ...string) types.GomegaMatcher {
	return &vsGatewayMatcher{expectedGateways: targetGw}
}

type vsGatewayMatcher struct {
	expectedGateways []string
}

func (v *vsGatewayMatcher) Match(actual any) (bool, error) {
	if actual == nil {
		return true, nil
	}

	vs, errDeref := deref[v1beta1.VirtualService](actual)
	if errDeref != nil {
		return false, errDeref
	}

	return gomega.And(gomega.ContainElements(v.expectedGateways), gomega.HaveLen(len(v.expectedGateways))).Match(vs.Spec.GetGateways())
}

func (v *vsGatewayMatcher) FailureMessage(actual any) string {
	return format.Message(actual, "to be attached only to gateways", v.expectedGateways)
}

func (v *vsGatewayMatcher) NegatedFailureMessage(actual any) string {
	return format.Message(actual, "not to be attached only to gateways", v.expectedGateways)
}

// RouteToHost ensures that the VirtualService is attached to only specified gateways.
func RouteToHost(host string, port uint32) types.GomegaMatcher {
	return &vsDestinationMatcher{
		host: host,
		port: port,
	}
}

type vsDestinationMatcher struct {
	host string
	port uint32
}

func (v *vsDestinationMatcher) Match(actual any) (bool, error) {
	if actual == nil {
		return true, nil
	}

	vs, errDeref := deref[v1beta1.VirtualService](actual)
	if errDeref != nil {
		return false, errDeref
	}

	httpRouteMatcher := gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
		"Route": gomega.ContainElement(gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
			"Destination": gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
				"Host": gomega.Equal(v.host),
				"Port": gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Number": gomega.Equal(v.port),
				})),
			})),
		}))),
	}))

	return gomega.ContainElement(httpRouteMatcher).Match(vs.Spec.GetHttp())
}

func (v *vsDestinationMatcher) FailureMessage(actual any) string {
	vs, errDeref := deref[v1beta1.VirtualService](actual)
	if errDeref != nil {
		return errDeref.Error()
	}

	return format.Message(vs.Spec.GetHttp(), "to be routing to ", v.host, ":", v.port)
}

func (v *vsDestinationMatcher) NegatedFailureMessage(actual any) string {
	return format.Message(actual, "not to be routing to ", v.host, ":", v.port)
}
