package routing

import (
	"context"
	"fmt"

	"github.com/opendatahub-io/odh-platform/pkg/cluster"
	"github.com/opendatahub-io/odh-platform/pkg/metadata"
	"github.com/opendatahub-io/odh-platform/pkg/spi"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// HandleResourceDeletion handles the removal of dependent resources when the target resource is being deleted.
func (r *PlatformRoutingReconciler) HandleResourceDeletion(ctx context.Context, sourceRes *unstructured.Unstructured) (ctrl.Result, error) {
	exportModes, found := extractExportModes(sourceRes)
	if !found {
		r.log.Info("No export modes found, skipping deletion logic", "sourceRes", sourceRes)

		return ctrl.Result{}, nil
	}

	r.log.Info("Handling deletion of dependent resources", "sourceRes", sourceRes)

	for _, exportMode := range exportModes {
		if err := r.deleteResourcesForExportMode(ctx, sourceRes, exportMode); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to delete resources for export mode %s: %w", exportMode, err)
		}
	}

	return removeFinalizerAndUpdate(ctx, r.Client, sourceRes)
}

// deleteResourcesForExportMode deletes resources based on the given export mode.
func (r *PlatformRoutingReconciler) deleteResourcesForExportMode(ctx context.Context, target *unstructured.Unstructured, exportMode spi.RouteType) error {
	switch exportMode {
	case spi.ExternalRoute:
		return r.deleteResourcesByLabels(ctx, target, metadata.ExternalGVKs())
	case spi.PublicRoute:
		return r.deleteResourcesByLabels(ctx, target, metadata.PublicGVKs())
	}

	return nil
}

func (r *PlatformRoutingReconciler) deleteResourcesByLabels(ctx context.Context, target *unstructured.Unstructured, gvkList []schema.GroupVersionKind) error {
	ownerName := target.GetName()
	ownerKind := target.GetObjectKind().GroupVersionKind().Kind

	labelSelector := client.MatchingLabels{
		metadata.Labels.OwnerName: ownerName,
		metadata.Labels.OwnerKind: ownerKind,
	}

	var resourcesToDelete []*unstructured.Unstructured

	for _, gvk := range gvkList {
		resourceList := &unstructured.UnstructuredList{}
		resourceList.SetGroupVersionKind(gvk)
		err := r.Client.List(ctx, resourceList, client.InNamespace(r.config.GatewayNamespace), labelSelector)

		if err != nil {
			return fmt.Errorf("error listing resources: %w", err)
		}

		for _, resource := range resourceList.Items {
			resourceCopy := resource.DeepCopy()
			resourcesToDelete = append(resourcesToDelete, resourceCopy)
		}
	}

	var deletionErr error
	if err := cluster.Delete(ctx, r.Client, resourcesToDelete); err != nil {
		deletionErr = fmt.Errorf("failed to delete resources: %w", err)
	}

	return deletionErr
}

func removeFinalizerAndUpdate(ctx context.Context, cli client.Client, sourceRes *unstructured.Unstructured) (ctrl.Result, error) {
	finalizer := metadata.Finalizers.Routing

	if controllerutil.ContainsFinalizer(sourceRes, finalizer) {
		controllerutil.RemoveFinalizer(sourceRes, finalizer)

		if err := cli.Update(ctx, sourceRes); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update resource to remove finalizer: %w", err)
		}
	}

	return ctrl.Result{}, nil
}
