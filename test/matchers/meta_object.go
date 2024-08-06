package matchers

import (
	"errors"
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func HaveAnnotations(annotationsKV ...any) types.GomegaMatcher {
	annotations, err := extractKeyValues(annotationsKV)
	if err != nil {
		ginkgo.Fail(fmt.Sprintf("failed to extract annotations: %v", err))
	}

	return &annotationsMatcher{expectedAnnotations: annotations}
}

func extractKeyValues(keyValuePairs []any) (map[string]any, error) {
	lenKV := len(keyValuePairs)
	if lenKV%2 != 0 {
		return nil, fmt.Errorf("passed elements should be in key/value pairs, but got %d elements", lenKV)
	}

	kvMap := make(map[string]any, lenKV%2)

	for i := 0; i < lenKV; i += 2 { //nolint:varnamelen //reason: i is a common name for loop counters
		key, ok := keyValuePairs[i].(string)
		if !ok {
			return nil, fmt.Errorf("passed key %T, expected string", keyValuePairs[i])
		}

		kvMap[key] = keyValuePairs[i+1]
	}

	return kvMap, nil
}

type annotationsMatcher struct {
	expectedAnnotations map[string]any
}

func (r *annotationsMatcher) Match(actual any) (bool, error) {
	metaObj, ok := actual.(metav1.Object)
	if !ok {
		return false, fmt.Errorf("object does not implement metav1.Object, got type %T", actual)
	}

	var matchErrs error

	succeed := true
	annotations := metaObj.GetAnnotations()

	for key, value := range r.expectedAnnotations {
		success, err := gomega.HaveKeyWithValue(key, value).Match(annotations)
		if !success {
			succeed = false
		}

		matchErrs = errors.Join(matchErrs, err)
	}

	return succeed, matchErrs
}

func (r *annotationsMatcher) FailureMessage(actual any) string {
	return format.Message(actual, "to have annotations", r.expectedAnnotations)
}

func (r *annotationsMatcher) NegatedFailureMessage(actual any) string {
	return format.Message(actual, "not to be attached to service named", r.expectedAnnotations)
}

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
