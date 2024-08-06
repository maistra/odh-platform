package metadata

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// WithOwnerLabels sets the owner labels on the object based on source object.
// Those labels can be used to find all related resources across the cluster
// which are owned by the source object by leveraging label selectors.
// This can be particularly useful for garbage collection when source object is namespace-scoped
// and related resources are created in a different namespace or are cluster-scoped.
func WithOwnerLabels(source client.Object) Options {
	ownerName := source.GetName()
	if source.GetNamespace() != "" {
		ownerName = source.GetNamespace() + "-" + ownerName
	}

	ownerKind := source.GetObjectKind().GroupVersionKind().Kind

	return WithLabels(
		Labels.OwnerName, ownerName,
		Labels.OwnerKind, ownerKind,
	)
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
