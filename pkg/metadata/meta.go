package metadata

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Options allows to add additional settings for the object being created through a chain
// of functions which are applied on metav1.Object before actual resource creation.
type Options func(obj metav1.Object) error

func ApplyMetaOptions(obj metav1.Object, opts ...Options) error {
	for _, opt := range opts {
		if err := opt(obj); err != nil {
			return err
		}
	}

	return nil
}

func WithLabels(labels ...string) Options {
	return func(obj metav1.Object) error {
		labelsMap, err := extractKeyValues(labels)
		if err != nil {
			return fmt.Errorf("failed unable to set labels: %w", err)
		}

		obj.SetLabels(labelsMap)

		return nil
	}
}

func WithAnnotations(annotationKeyValue ...string) Options {
	return func(obj metav1.Object) error {
		annotationsMap, err := extractKeyValues(annotationKeyValue)
		if err != nil {
			return fmt.Errorf("failed to set labels: %w", err)
		}

		obj.SetAnnotations(annotationsMap)

		return nil
	}
}

func extractKeyValues(keyValuePairs []string) (map[string]string, error) {
	lenKV := len(keyValuePairs)
	if lenKV%2 != 0 {
		return nil, fmt.Errorf("passed elements should be in key/value pairs, but got %d elements", lenKV)
	}

	kvMap := make(map[string]string, lenKV%2)
	for i := 0; i < lenKV; i += 2 {
		kvMap[keyValuePairs[i]] = keyValuePairs[i+1]
	}

	return kvMap, nil
}
