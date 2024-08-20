package routing

import (
	"context"
	"fmt"

	"github.com/opendatahub-io/odh-platform/pkg/metadata/labels"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// HandleResourceDeletion handles the removal of dependent resources when the target resource is being deleted.
func (r *PlatformRoutingController) HandleResourceDeletion(ctx context.Context, sourceRes *unstructured.Unstructured) (ctrl.Result, error) {
	exportModes, found := extractExportModes(sourceRes)
	if !found {
		r.log.Info("No export modes found, skipping deletion logic", "sourceRes", sourceRes)

		return ctrl.Result{}, nil
	}

	r.log.Info("Handling deletion of dependent resources", "sourceRes", sourceRes)

	for _, exportMode := range exportModes {
		if err := r.deleteOwnedResources(ctx, sourceRes, routingResourceGVKs(exportMode)); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to delete resources for export mode %s: %w", exportMode, err)
		}
	}

	return removeFinalizer(ctx, r.Client, sourceRes)
}

func (r *PlatformRoutingController) deleteOwnedResources(ctx context.Context, target *unstructured.Unstructured, gvkList []schema.GroupVersionKind) error {
	deleteOptions := []client.DeleteAllOfOption{
		client.InNamespace(r.config.GatewayNamespace),
		labels.MatchingLabels(
			labels.OwnerName(target.GetName()),
			labels.OwnerKind(target.GetObjectKind().GroupVersionKind().Kind),
			labels.OwnerUID(target.GetUID()),
		),
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
func removeFinalizer(ctx context.Context, cli client.Client, sourceRes *unstructured.Unstructured) (ctrl.Result, error) {
	finalizer := finalizerName

	if controllerutil.ContainsFinalizer(sourceRes, finalizer) {
		controllerutil.RemoveFinalizer(sourceRes, finalizer)

		if err := cli.Update(ctx, sourceRes); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	return ctrl.Result{}, nil
}
