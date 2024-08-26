package routingctrl

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getExportedServices(ctx context.Context, cli client.Client, labels map[string]string, target *unstructured.Unstructured) ([]corev1.Service, error) {
	listOpts := []client.ListOption{
		client.InNamespace(target.GetNamespace()),
		client.MatchingLabels(labels),
	}

	var exportedSvcList *corev1.ServiceList

	// It is possible that the exported services are not yet created when we first receive CREATE event for watched CR
	// and trigger reconcile. Retry to see if they show up in the cluster.
	if errRetry := retry.OnError(retry.DefaultBackoff, isExportedServiceNotFoundError, func() error {
		exportedSvcList = &corev1.ServiceList{}
		if errList := cli.List(ctx, exportedSvcList, listOpts...); errList != nil {
			return fmt.Errorf("could not list exported services: %w", errList)
		}

		if len(exportedSvcList.Items) == 0 {
			return &ExportedServiceNotFoundError{target: target}
		}

		return nil
	}); errRetry != nil {
		return nil, fmt.Errorf("failed retrying to fetch exported services: %w", errRetry)
	}

	return exportedSvcList.Items, nil
}

// ExportedServiceNotFoundError represents a custom error type for missing exported services.
type ExportedServiceNotFoundError struct {
	target client.Object
}

func (e *ExportedServiceNotFoundError) Error() string {
	return fmt.Sprintf("no exported services found for target %s/%s (%s)", e.target.GetNamespace(), e.target.GetName(), e.target.GetObjectKind().GroupVersionKind().String())
}

func isExportedServiceNotFoundError(err error) bool {
	return errors.Is(err, &ExportedServiceNotFoundError{})
}
