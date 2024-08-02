package routing

import (
	"context"
	"fmt"

	"github.com/opendatahub-io/odh-platform/controllers"
	"github.com/opendatahub-io/odh-platform/pkg/env"
	"github.com/opendatahub-io/odh-platform/pkg/spi"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

func (r *PlatformRoutingReconciler) reconcileResources(ctx context.Context, target *unstructured.Unstructured) error {
	exportMode, exportModeFound := target.GetAnnotations()[controllers.AnnotationRoutingExportMode]
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
		GatewayNamespace:  env.GetGatewayNamespace(),
		Domain:            "app-crc.testing", // TODO: Read from where?

		IngressSelectorLabel: env.GetIngressSelectorKey(),
		IngressSelectorValue: env.GetIngressSelectorValue(),
		IngressService:       env.GetGatewayService(),
	}

	targetKey := types.NamespacedName{Namespace: target.GetNamespace(), Name: target.GetName()}
	if _, err := r.templateLoader.Load(ctx, spi.RouteType(exportMode), targetKey, templateData); err != nil {
		return fmt.Errorf("could not load templates for type %s: %w", exportMode, err)
	}

	return nil
}
