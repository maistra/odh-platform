package routingctrl

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"github.com/opendatahub-io/odh-platform/pkg/cluster"
	"github.com/opendatahub-io/odh-platform/pkg/config"
	"github.com/opendatahub-io/odh-platform/pkg/metadata"
	"github.com/opendatahub-io/odh-platform/pkg/metadata/annotations"
	"github.com/opendatahub-io/odh-platform/pkg/metadata/labels"
	"github.com/opendatahub-io/odh-platform/pkg/spi"
	"github.com/opendatahub-io/odh-platform/pkg/unstruct"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *Controller) createRoutingResources(ctx context.Context, target *unstructured.Unstructured) error {
	exportModes := extractExportModes(target, r.log)

	if len(exportModes) == 0 {
		r.log.Info("No export mode found for target")

		return propagateHostsToWatchedCR(target, spi.RoutingTemplateData{}, r.log)
	}

	r.log.Info("Reconciling resources for target", "target", target)

	renderedSelectors, errLables := config.ResolveSelectors(r.component.ServiceSelector, target)
	if errLables != nil {
		return fmt.Errorf("could not render labels for ServiceSelector %v. Error %w", r.component.ServiceSelector, errLables)
	}

	exportedServices, errSvcGet := getExportedServices(ctx, r.Client, renderedSelectors, target)
	if errSvcGet != nil {
		if errors.Is(errSvcGet, &ExportedServiceNotFoundError{}) {
			r.log.Info("no exported services found for target", "target", target)

			return nil
		}

		return errSvcGet
	}

	domain, errDomain := cluster.GetDomain(ctx, r.Client)
	if errDomain != nil {
		return fmt.Errorf("could not get domain: %w", errDomain)
	}

	var errSvcExport []error

	for i := range exportedServices {
		if errExport := r.exportService(ctx, target, &exportedServices[i], domain); errExport != nil {
			errSvcExport = append(errSvcExport, errExport)
		}
	}

	return errors.Join(errSvcExport...)
}

func (r *Controller) exportService(ctx context.Context, target *unstructured.Unstructured, exportedSvc *corev1.Service, domain string) error {
	exportModes := extractExportModes(target, r.log)

	templateData := spi.RoutingTemplateData{
		PlatformRoutingConfiguration: r.config,
		PublicServiceName:            exportedSvc.GetName() + "-" + exportedSvc.GetNamespace(),
		ServiceName:                  exportedSvc.GetName(),
		ServiceNamespace:             exportedSvc.GetNamespace(),
		ServiceTargetPort:            exportedSvc.Spec.Ports[0].TargetPort.String(),
		Domain:                       domain,
	}

	// To establish ownership for watched component
	ownershipLabels := append(labels.AsOwner(target), labels.AppManagedBy("odh-routing-controller"))

	targetKey := client.ObjectKeyFromObject(target)

	for _, exportMode := range exportModes {
		resources, err := r.templateLoader.Load(ctx, exportMode, targetKey, templateData)
		if err != nil {
			return fmt.Errorf("could not load templates for type %s: %w", exportMode, err)
		}

		ownershipLabels = append(ownershipLabels, labels.ExportType(exportMode))
		if errApply := unstruct.Apply(ctx, r.Client, resources, ownershipLabels...); errApply != nil {
			return fmt.Errorf("could not apply routing resources for type %s: %w", exportMode, errApply)
		}
	}

	return propagateHostsToWatchedCR(target, templateData, r.log)
}

func propagateHostsToWatchedCR(target *unstructured.Unstructured, data spi.RoutingTemplateData, log logr.Logger) error {
	exportModes := extractExportModes(target, log)

	annotationsToKeep := make(map[spi.RouteType]bool)

	var metaOptions []metadata.Option

	// TODO(mvp): put the logic of creating host names into a single place
	for _, exportMode := range exportModes {
		switch exportMode {
		case spi.ExternalRoute:
			externalAddress := annotations.RoutingAddressesExternal(fmt.Sprintf("%s-%s.%s", data.ServiceName, data.ServiceNamespace, data.Domain))
			metaOptions = append(metaOptions, externalAddress)
			annotationsToKeep[spi.ExternalRoute] = true
		case spi.PublicRoute:
			publicAddresses := annotations.RoutingAddressesPublic(fmt.Sprintf("%[1]s.%[2]s;%[1]s.%[2]s.svc;%[1]s.%[2]s.svc.cluster.local", data.PublicServiceName, data.GatewayNamespace))
			metaOptions = append(metaOptions, publicAddresses)
			annotationsToKeep[spi.PublicRoute] = true
		}
	}

	// Remove annotations that should not be present
	targetAnnotations := target.GetAnnotations()
	if targetAnnotations == nil {
		targetAnnotations = make(map[string]string)
	}

	if !annotationsToKeep[spi.ExternalRoute] {
		delete(targetAnnotations, annotations.RoutingAddressesExternal("").Key())
	}

	if !annotationsToKeep[spi.PublicRoute] {
		delete(targetAnnotations, annotations.RoutingAddressesPublic("").Key())
	}

	target.SetAnnotations(targetAnnotations)

	// Apply the meta options for the annotations that should be present
	metadata.ApplyMetaOptions(target, metaOptions...)

	return nil
}

func extractExportModes(target *unstructured.Unstructured, log logr.Logger) []spi.RouteType {
	exportModes, exportModeFound := target.GetAnnotations()[annotations.RoutingExportMode("").Key()]
	if !exportModeFound {
		return nil
	}

	exportModesSplit := strings.Split(exportModes, ";")
	validRouteTypes := make([]spi.RouteType, 0, len(exportModesSplit))

	for _, exportMode := range exportModesSplit {
		routeType := spi.RouteType(strings.TrimSpace(exportMode))
		if spi.IsValidRouteType(routeType) {
			validRouteTypes = append(validRouteTypes, routeType)
		} else {
			log.Info("Invalid route type found",
				"invalidRouteType", routeType,
				"resourceName", target.GetName(),
				"resourceNamespace", target.GetNamespace())
		}
	}

	return validRouteTypes
}
