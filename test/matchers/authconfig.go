package matchers

import (
	"fmt"

	authorinov1beta2 "github.com/kuadrant/authorino/api/v1beta2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
)

// HaveKubernetesTokenReview is a custom matcher to verify Kubernetes Token Review configuration in AuthConfigs.
func HaveKubernetesTokenReview() types.GomegaMatcher {
	return &kubernetesTokenReviewMatcher{}
}

type kubernetesTokenReviewMatcher struct{}

func (matcher *kubernetesTokenReviewMatcher) Match(actual any) (bool, error) {
	if actual == nil {
		return false, nil
	}

	authConfig, err := deref[authorinov1beta2.AuthConfig](actual)
	if err != nil {
		return false, fmt.Errorf("expected AuthConfig. Got:\n%s", format.Object(actual, 1))
	}

	authMethod, found := authConfig.Spec.Authentication["kubernetes-user"]
	if !found {
		return false, nil
	}

	return authMethod.KubernetesTokenReview != nil, nil
}

func (matcher *kubernetesTokenReviewMatcher) FailureMessage(actual any) string {
	return format.Message(actual, "to have Kubernetes Token Review configured for kubernetes-user authentication")
}

func (matcher *kubernetesTokenReviewMatcher) NegatedFailureMessage(actual any) string {
	return format.Message(actual, "not to have Kubernetes Token Review configured for kubernetes-user authentication")
}

// HaveAuthenticationMethod is a custom matcher to verify the presence of an authentication method in AuthConfigs.
func HaveAuthenticationMethod(method string) types.GomegaMatcher {
	return &authConfigMethodMatcher{expectedMethod: method}
}

type authConfigMethodMatcher struct {
	expectedMethod string
}

func (matcher *authConfigMethodMatcher) Match(actual any) (bool, error) {
	if actual == nil {
		return false, nil
	}

	authConfig, err := deref[authorinov1beta2.AuthConfig](actual)
	if err != nil {
		return false, fmt.Errorf("expected AuthConfig. Got:\n%s", format.Object(actual, 1))
	}

	return gomega.HaveKey(matcher.expectedMethod).Match(authConfig.Spec.Authentication)
}

func (matcher *authConfigMethodMatcher) FailureMessage(actual any) string {
	return format.Message(actual, "to have authentication method", matcher.expectedMethod)
}

func (matcher *authConfigMethodMatcher) NegatedFailureMessage(actual any) string {
	return format.Message(actual, "not to have authentication method", matcher.expectedMethod)
}
