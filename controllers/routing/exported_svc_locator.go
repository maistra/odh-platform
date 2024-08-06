package routing

import (
	"context"
	"fmt"

	"github.com/opendatahub-io/odh-platform/pkg/metadata"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getExportedService(ctx context.Context, cli client.Client, target *unstructured.Unstructured) (*corev1.Service, error) {
	exportedSvcList := &corev1.ServiceList{}
	listOpts := []client.ListOption{
		client.InNamespace(target.GetNamespace()),
		client.MatchingLabels{metadata.Labels.RoutingExported: "true"},
	}

	if errList := cli.List(ctx, exportedSvcList, listOpts...); errList != nil {
		return nil, fmt.Errorf("could not list exported services: %w", errList)
	}

	if len(exportedSvcList.Items) == 0 {
		return nil, &NoExportedServicesError{Target: target}
	}

	return &exportedSvcList.Items[0], nil
}

// NoExportedServicesError represents a custom error type for missing exported services.
type NoExportedServicesError struct {
	Target client.Object
}

func (e *NoExportedServicesError) Error() string {
	return fmt.Sprintf("no exported services found for target %s/%s (%s)", e.Target.GetNamespace(), e.Target.GetName(), e.Target.GetObjectKind().GroupVersionKind().String())
}
