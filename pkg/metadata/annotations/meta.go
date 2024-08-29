package annotations

import (
	"strings"

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

// GetExportModes returns a slice of enabled export modes from the object's annotations.
func GetExportModes(obj metav1.Object) []string {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return nil
	}

	var modes []string

	for key, value := range annotations {
		if strings.HasPrefix(key, RoutingExportModePrefix) {
			mode := strings.TrimPrefix(key, RoutingExportModePrefix)

			if value == "true" {
				modes = append(modes, mode)
			}
		}
	}

	return modes
}
