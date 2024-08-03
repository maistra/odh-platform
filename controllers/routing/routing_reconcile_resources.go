package routing

import (
	"context"
	"fmt"

	"github.com/opendatahub-io/odh-platform/pkg/config"
	"github.com/opendatahub-io/odh-platform/pkg/metadata"
	"github.com/opendatahub-io/odh-platform/pkg/spi"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

func (r *PlatformRoutingReconciler) reconcileResources(ctx context.Context, target *unstructured.Unstructured) error {
	exportMode, exportModeFound := target.GetAnnotations()[metadata.Annotations.RoutingExportMode]
	if !exportModeFound {
		return nil
	}

	// TODO: lookup service owned by target with "ExportableService" annotation
	// TODO: use label instead, then we could do a service label search?
	// routing.opendatahub.io/exposable=true && app.kubernetes.io/part-of=target.Name

	templateData := spi.RoutingTemplateData{
		PublicServiceName: "registry-office", // TODO: serviceName - serviceNamespace
		ServiceName:       "registry",        // TODO: lookupService.GetName()
		ServiceNamespace:  target.GetNamespace(),
		GatewayNamespace:  config.GetGatewayNamespace(),
		Domain:            "app-crc.testing", // TODO: Read from where?

		IngressSelectorLabel: config.GetIngressSelectorKey(),
		IngressSelectorValue: config.GetIngressSelectorValue(),
		IngressService:       config.GetGatewayService(),
	}

	targetKey := types.NamespacedName{Namespace: target.GetNamespace(), Name: target.GetName()}
	if _, err := r.templateLoader.Load(ctx, spi.RouteType(exportMode), targetKey, templateData); err != nil {
		return fmt.Errorf("could not load templates for type %s: %w", exportMode, err)
	}

	return nil
}
