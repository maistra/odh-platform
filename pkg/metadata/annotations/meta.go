package annotations

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

func Remove(annotation Annotation) OptionFunc {
	return func(obj metav1.Object) {
		existingAnnotations := obj.GetAnnotations()
		if existingAnnotations == nil {
			existingAnnotations = make(map[string]string)
		}

		delete(existingAnnotations, annotation.Key())

		obj.SetAnnotations(existingAnnotations)
	}
}

type OptionFunc func(obj metav1.Object)

func (f OptionFunc) ApplyToMeta(obj metav1.Object) {
	f(obj)
}
