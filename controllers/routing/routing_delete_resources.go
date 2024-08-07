package routing

import (
	"context"
	"errors"
	"fmt"

	"github.com/opendatahub-io/odh-platform/pkg/cluster"
	"github.com/opendatahub-io/odh-platform/pkg/metadata"
	"github.com/opendatahub-io/odh-platform/pkg/spi"
	openshiftroutev1 "github.com/openshift/api/route/v1"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// HandleResourceDeletion handles the removal of dependent resources when the target resource is being deleted.
func (r *PlatformRoutingReconciler) HandleResourceDeletion(ctx context.Context, target *unstructured.Unstructured) error {
	exportModes, found := extractExportModes(target)
	if !found {
		r.log.Info("No export modes found, skipping deletion logic", "target", target)

		return nil
	}

	exportedSvc, errSvcGet := getExportedService(ctx, r.Client, target)
	if errSvcGet != nil {
		if errors.Is(errSvcGet, &NoExportedServicesError{}) {
			r.log.Info("no exported service found for target", "target", target)

			return nil
		}

		return errSvcGet
	}

	publicSvcName := exportedSvc.GetName() + "-" + exportedSvc.GetNamespace()

	r.log.Info("Handling deletion of dependent resources", "target", target)

	// Iterate over the export modes and delete corresponding resources
	for _, exportMode := range exportModes {
		if err := r.deleteResourcesForExportMode(ctx, target, publicSvcName, exportMode); err != nil {
			return fmt.Errorf("failed to delete resources for export mode %s: %w", exportMode, err)
		}
	}

	return nil
}

// deleteResourcesForExportMode deletes resources based on the given export mode.
func (r *PlatformRoutingReconciler) deleteResourcesForExportMode(ctx context.Context, target *unstructured.Unstructured, svcName string, exportMode spi.RouteType) error {
	switch exportMode {
	case spi.ExternalRoute:
		return r.deleteExternalResources(ctx, target, svcName)
	case spi.PublicRoute:
		return r.deletePublicResources(ctx, target, svcName)
	}

	return nil
}

func (r *PlatformRoutingReconciler) deleteExternalResources(ctx context.Context, target *unstructured.Unstructured, svcName string) error {
	var deletionErr error

	routeResource := &unstructured.Unstructured{}
	routeResource.SetGroupVersionKind(openshiftroutev1.SchemeGroupVersion.WithKind("Route"))
	routeResource.SetName(svcName + "-route")
	routeResource.SetNamespace(r.config.GatewayNamespace)

	virtualServiceResource := &unstructured.Unstructured{}
	virtualServiceResource.SetGroupVersionKind(istionetworkingv1beta1.SchemeGroupVersion.WithKind("VirtualService"))
	virtualServiceResource.SetName(svcName + "-ingress")
	virtualServiceResource.SetNamespace(r.config.GatewayNamespace)

	resources := []*unstructured.Unstructured{routeResource, virtualServiceResource}

	if err := cluster.Delete(ctx, r.Client, resources, metadata.WithOwnerLabels(target)); err != nil {
		deletionErr = fmt.Errorf("failed to delete external resources: %w", err)
		r.log.Error(deletionErr, "Error deleting external resources")
	}

	return deletionErr
}

func (r *PlatformRoutingReconciler) deletePublicResources(ctx context.Context, target *unstructured.Unstructured, svcName string) error {
	var deletionErr error

	serviceResource := &unstructured.Unstructured{}
	serviceResource.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Service",
	})
	serviceResource.SetName(svcName)
	serviceResource.SetNamespace(r.config.GatewayNamespace)

	gatewayResource := &unstructured.Unstructured{}
	gatewayResource.SetGroupVersionKind(istionetworkingv1beta1.SchemeGroupVersion.WithKind("Gateway"))
	gatewayResource.SetName(svcName)
	gatewayResource.SetNamespace(r.config.GatewayNamespace)

	virtualServiceResource := &unstructured.Unstructured{}
	virtualServiceResource.SetGroupVersionKind(istionetworkingv1beta1.SchemeGroupVersion.WithKind("VirtualService"))
	virtualServiceResource.SetName(svcName)
	virtualServiceResource.SetNamespace(r.config.GatewayNamespace)

	destinationRuleResource := &unstructured.Unstructured{}
	destinationRuleResource.SetGroupVersionKind(istionetworkingv1beta1.SchemeGroupVersion.WithKind("DestinationRule"))
	destinationRuleResource.SetName(svcName)
	destinationRuleResource.SetNamespace(r.config.GatewayNamespace)

	resources := []*unstructured.Unstructured{
		serviceResource,
		gatewayResource,
		virtualServiceResource,
		destinationRuleResource,
	}

	if err := cluster.Delete(ctx, r.Client, resources, metadata.WithOwnerLabels(target)); err != nil {
		deletionErr = fmt.Errorf("failed to delete public resources: %w", err)
		r.log.Error(deletionErr, "Error deleting public resources")
	}

	return deletionErr
}
