package matchers

import (
	"fmt"

	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func HaveOwnerReference(expectedOwner metav1.OwnerReference) types.GomegaMatcher {
	return &ownerReferenceMatcher{expectedOwner: expectedOwner}
}

type ownerReferenceMatcher struct {
	expectedOwner metav1.OwnerReference
}

func (matcher *ownerReferenceMatcher) Match(actual any) (bool, error) {
	if actual == nil {
		return false, nil
	}

	metaObject, ok := actual.(metav1.Object)
	if !ok {
		return false, fmt.Errorf("expected metav1.Object. Got:\n%s", format.Object(actual, 1))
	}

	ownerReferences := metaObject.GetOwnerReferences()
	if len(ownerReferences) == 0 {
		return false, nil
	}

	for _, owner := range ownerReferences {
		if owner.Name == matcher.expectedOwner.Name &&
			owner.UID == matcher.expectedOwner.UID &&
			owner.BlockOwnerDeletion == nil {
			return true, nil
		}
	}

	return false, nil
}

func (matcher *ownerReferenceMatcher) FailureMessage(actual any) string {
	return format.Message(actual, "to have owner reference with Name", matcher.expectedOwner.Name,
		"UID", matcher.expectedOwner.UID, "and BlockOwnerDeletion as nil")
}

func (matcher *ownerReferenceMatcher) NegatedFailureMessage(actual any) string {
	return format.Message(actual, "not to have owner reference with Name", matcher.expectedOwner.Name,
		"UID", matcher.expectedOwner.UID, "and BlockOwnerDeletion as nil")
}
