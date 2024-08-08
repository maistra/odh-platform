package routing

import (
	"context"
	"errors"
	"fmt"

	"github.com/opendatahub-io/odh-platform/pkg/metadata"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getExportedServices(ctx context.Context, cli client.Client, target *unstructured.Unstructured) ([]corev1.Service, error) {
	ownerName := target.GetName()
	ownerKind := target.GetObjectKind().GroupVersionKind().Kind

	listOpts := []client.ListOption{
		client.InNamespace(target.GetNamespace()),
		// TODO(mvp): centralize label creation
		client.MatchingLabels{
			metadata.Labels.RoutingExported: "true",
			metadata.Labels.OwnerName:       ownerName,
			metadata.Labels.OwnerKind:       ownerKind,
		},
	}

	var exportedSvcList *corev1.ServiceList

	// It is possible that the exported services are not yet created when we first receive CREATE event for watched CR
	// and trigger reconcile. Retry to see if they show up in the cluster.
	if errRetry := retry.OnError(retry.DefaultBackoff, isNoExportedServicesError, func() error {
		exportedSvcList = &corev1.ServiceList{}
		if errList := cli.List(ctx, exportedSvcList, listOpts...); errList != nil {
			return fmt.Errorf("could not list exported services: %w", errList)
		}

		if len(exportedSvcList.Items) == 0 {
			return &NoExportedServicesError{Target: target}
		}

		return nil
	}); errRetry != nil {
		return nil, fmt.Errorf("failed retrying to fetch exported services: %w", errRetry)
	}

	return exportedSvcList.Items, nil
}

// NoExportedServicesError represents a custom error type for missing exported services.
type NoExportedServicesError struct {
	Target client.Object
}

func (e *NoExportedServicesError) Error() string {
	return fmt.Sprintf("no exported services found for target %s/%s (%s)", e.Target.GetNamespace(), e.Target.GetName(), e.Target.GetObjectKind().GroupVersionKind().String())
}

func isNoExportedServicesError(err error) bool {
	return errors.Is(err, &NoExportedServicesError{})
}
