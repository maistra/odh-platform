package routing

import (
	"context"
	"fmt"

	"github.com/opendatahub-io/odh-platform/pkg/metadata"
	"github.com/opendatahub-io/odh-platform/pkg/spi"
	"github.com/opendatahub-io/odh-platform/pkg/metadata/labels"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *PlatformRoutingController) removeUnusedRoutingResources(ctx context.Context, target *unstructured.Unstructured) error {
	exportModes := extractExportModes(target)
	unusedRouteTypes := spi.UnusedRouteTypes(exportModes)

	if len(unusedRouteTypes) == 0 {
		// no unused route types to remove resources for
		return nil
	}

	var gvks []schema.GroupVersionKind
	for _, unusedRouteType := range unusedRouteTypes {
		gvks = append(gvks, routingResourceGVKs(unusedRouteType)...)
	}

	return r.deleteOwnedResources(ctx, target, unusedRouteTypes, gvks)
}

func (r *PlatformRoutingController) handleResourceDeletion(ctx context.Context, sourceRes *unstructured.Unstructured) error {
	exportModes := extractExportModes(sourceRes)
	if len(exportModes) == 0 {
		r.log.Info("No export modes found, skipping deletion logic", "sourceRes", sourceRes)

		return nil
	}

	r.log.Info("Handling deletion of dependent resources", "sourceRes", sourceRes)

	var gvks []schema.GroupVersionKind
	for _, exportMode := range exportModes {
		gvks = append(gvks, routingResourceGVKs(exportMode)...)
	}

	if err := r.deleteOwnedResources(ctx, sourceRes, exportModes, gvks); err != nil {
		return fmt.Errorf("failed to delete resources: %w", err)
	}

	return removeFinalizer(ctx, r.Client, sourceRes)
}

func (r *PlatformRoutingController) deleteOwnedResources(ctx context.Context,
	target *unstructured.Unstructured,
	exportModes []spi.RouteType,
	gvks []schema.GroupVersionKind) error {
	ownerName := target.GetName()
	ownerKind := target.GetObjectKind().GroupVersionKind().Kind
	ownerUID := string(target.GetUID())

	exportTypeValues := make([]string, len(exportModes))
	for i, mode := range exportModes {
		exportTypeValues[i] = string(mode)
	}

	requirement, err := labels.NewRequirement(metadata.Labels.ExportType, selection.In, exportTypeValues)
	if err != nil {
		return fmt.Errorf("failed to create label requirement: %w", err)
	}

	routeTypes := labels.NewSelector().Add(*requirement)
	resourceOwnerLabels := client.MatchingLabels{
		metadata.Labels.OwnerName: ownerName,
		metadata.Labels.OwnerKind: ownerKind,
		metadata.Labels.OwnerUID:  ownerUID,
	}

	deleteOptions := []client.DeleteAllOfOption{
		client.InNamespace(r.config.GatewayNamespace),
		labels.MatchingLabels(
			labels.OwnerName(target.GetName()),
			labels.OwnerKind(target.GetObjectKind().GroupVersionKind().Kind),
			labels.OwnerUID(target.GetUID()),
		),
		client.MatchingLabelsSelector{Selector: routeTypes},
	}

	for _, gvk := range gvks {
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
