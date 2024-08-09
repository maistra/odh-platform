package routing

import (
	"context"
	"fmt"

	"github.com/opendatahub-io/odh-platform/pkg/metadata"
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
		if err := r.deleteResourcesByLabels(ctx, sourceRes, metadata.ResourceGVKs(exportMode)); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to delete resources for export mode %s: %w", exportMode, err)
		}
	}

	return removeFinalizerAndUpdate(ctx, r.Client, sourceRes)
}

func (r *PlatformRoutingReconciler) deleteResourcesByLabels(ctx context.Context, target *unstructured.Unstructured, gvkList []schema.GroupVersionKind) error {
	ownerName := target.GetName()
	ownerKind := target.GetObjectKind().GroupVersionKind().Kind
	ownerUID := string(target.GetUID())

	resourceOwnerLabels := client.MatchingLabels{
		metadata.Labels.OwnerName: ownerName,
		metadata.Labels.OwnerKind: ownerKind,
		metadata.Labels.OwnerUID:  ownerUID,
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

func removeFinalizerAndUpdate(ctx context.Context, cli client.Client, sourceRes *unstructured.Unstructured) (ctrl.Result, error) {
	finalizer := metadata.Finalizers.Routing

	if controllerutil.ContainsFinalizer(sourceRes, finalizer) {
		controllerutil.RemoveFinalizer(sourceRes, finalizer)

		if err := cli.Update(ctx, sourceRes); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	return ctrl.Result{}, nil
}
