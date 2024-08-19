package routing

import (
	"context"
	"errors"
	"fmt"

	"github.com/opendatahub-io/odh-platform/pkg/metadata"
	"github.com/opendatahub-io/odh-platform/pkg/spi"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *PlatformRoutingController) removeUnusedRoutingResources(ctx context.Context, target *unstructured.Unstructured) error {
	if !target.GetDeletionTimestamp().IsZero() {
		return r.handleResourceDeletion(ctx, target)
	}

	exportModes := extractExportModes(target)
	unusedRouteTypes := spi.UnusedRouteTypes(exportModes)

	var errDeletion []error

	for _, unusedRouteType := range unusedRouteTypes {
		if errDel := r.deleteOwnedResources(ctx, target, unusedRouteType, routingResourceGVKs(unusedRouteType)); errDel != nil {
			errDeletion = append(errDeletion, errDel)
		}
	}

	return errors.Join(errDeletion...)
}

// handleResourceDeletion handles the removal of dependent resources when the target resource is being deleted.
func (r *PlatformRoutingController) handleResourceDeletion(ctx context.Context, sourceRes *unstructured.Unstructured) error {
	exportModes := extractExportModes(sourceRes)
	if len(exportModes) == 0 {
		r.log.Info("No export modes found, skipping deletion logic", "sourceRes", sourceRes)

		return nil
	}

	r.log.Info("Handling deletion of dependent resources", "sourceRes", sourceRes)

	for _, exportMode := range exportModes {
		if err := r.deleteOwnedResources(ctx, sourceRes, exportMode, routingResourceGVKs(exportMode)); err != nil {
			return fmt.Errorf("failed to delete resources for export mode %s: %w", exportMode, err)
		}
	}

	return removeFinalizer(ctx, r.Client, sourceRes)
}

func (r *PlatformRoutingController) deleteOwnedResources(ctx context.Context,
	target *unstructured.Unstructured,
	exportMode spi.RouteType,
	gvkList []schema.GroupVersionKind) error {
	ownerName := target.GetName()
	ownerKind := target.GetObjectKind().GroupVersionKind().Kind
	ownerUID := string(target.GetUID())
	exportType := string(exportMode)

	resourceOwnerLabels := client.MatchingLabels{
		metadata.Labels.OwnerName:  ownerName,
		metadata.Labels.OwnerKind:  ownerKind,
		metadata.Labels.OwnerUID:   ownerUID,
		metadata.Labels.ExportType: exportType,
	}

	deleteOptions := []client.DeleteAllOfOption{
		client.InNamespace(r.config.GatewayNamespace),
		resourceOwnerLabels,
	}

	for _, gvk := range gvkList {
		resource := &unstructured.Unstructured{}
		resource.SetGroupVersionKind(gvk)

		if err := r.Client.DeleteAllOf(ctx, resource, deleteOptions...); err != nil {
			return fmt.Errorf("failed to delete resources of kind %s: %w", gvk.Kind, err)
		}
	}

	return nil
}

// removeFinalizer is called after a successful cleanup, it removes the finalizer from the resource in the cluster.
func removeFinalizer(ctx context.Context, cli client.Client, sourceRes *unstructured.Unstructured) error {
	finalizer := metadata.Finalizers.Routing

	if controllerutil.ContainsFinalizer(sourceRes, finalizer) {
		controllerutil.RemoveFinalizer(sourceRes, finalizer)

		if err := cli.Update(ctx, sourceRes); err != nil {
			return fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	return nil
}
