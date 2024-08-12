package metadata

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Option allows to add additional settings for the object being created through a chain
// of functions which are applied on metav1.Object before actual resource creation.
type Option interface {
	ApplyToMeta(obj metav1.Object)
}

// KeyValue is a simple key-value pair interface used for metadata annotations and labels.
type KeyValue interface {
	Key() string
	Value() string
}

// Keys returns a list of keys from the provided key-value pairs.
// This can be used when you need keys from a list of annotations or labels.
func Keys(kvs ...KeyValue) []string {
	keys := make([]string, len(kvs))
	for i := range kvs {
		keys[i] = kvs[i].Key()
	}

	return keys
}

// ApplyMetaOptions applies a list of options to the provided object.
func ApplyMetaOptions(obj metav1.Object, opts ...Option) {
	for _, opt := range opts {
		opt.ApplyToMeta(obj)
	}
}
