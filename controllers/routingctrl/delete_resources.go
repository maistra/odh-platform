package routingctrl

import (
	"context"
	"fmt"

	"github.com/opendatahub-io/odh-platform/pkg/metadata"
	"github.com/opendatahub-io/odh-platform/pkg/metadata/labels"
	"github.com/opendatahub-io/odh-platform/pkg/spi"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *Controller) removeUnusedRoutingResources(ctx context.Context, target *unstructured.Unstructured) error {
	exportModes := extractExportModes(target, r.log)
	unusedRouteTypes := spi.UnusedRouteTypes(exportModes)

	if len(unusedRouteTypes) == 0 {
		// no unused route types to remove resources for
		return nil
	}

	gvks := routingResourceGVKs(unusedRouteTypes...)

	return r.deleteOwnedResources(ctx, target, unusedRouteTypes, gvks)
}

func (r *Controller) handleResourceDeletion(ctx context.Context, sourceRes *unstructured.Unstructured) error {
	exportModes := extractExportModes(sourceRes, r.log)
	if len(exportModes) == 0 {
		r.log.Info("No export modes found, skipping deletion logic", "sourceRes", sourceRes)

		return nil
	}

	r.log.Info("Handling deletion of dependent resources", "sourceRes", sourceRes)

	gvks := routingResourceGVKs(exportModes...)

	if err := r.deleteOwnedResources(ctx, sourceRes, exportModes, gvks); err != nil {
		return fmt.Errorf("failed to delete resources: %w", err)
	}

	return removeFinalizer(ctx, r.Client, sourceRes)
}

func (r *Controller) deleteOwnedResources(ctx context.Context,
	target *unstructured.Unstructured,
	exportModes []spi.RouteType,
	gvks []schema.GroupVersionKind) error {
	exportTypeValues := make([]string, len(exportModes))
	for i, mode := range exportModes {
		exportTypeValues[i] = string(mode)
	}

	requirement, err := k8slabels.NewRequirement(labels.ExportType("").Key(), selection.In, exportTypeValues)

	if err != nil {
		return fmt.Errorf("failed to create label requirement: %w", err)
	}

	routeTypes := k8slabels.NewSelector().Add(*requirement)
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