package annotations

import (
	"github.com/opendatahub-io/odh-platform/pkg/metadata"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Remove(annotation Annotation) metadata.OptionFunc {
	return func(obj metav1.Object) {
		existingAnnotations := obj.GetAnnotations()
		if existingAnnotations == nil {
			existingAnnotations = make(map[string]string)
		}

		delete(existingAnnotations, annotation.Key())

		obj.SetAnnotations(existingAnnotations)
	}
}
