package routing

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/opendatahub-io/odh-platform/pkg/cluster"
	"github.com/opendatahub-io/odh-platform/pkg/metadata"
	"github.com/opendatahub-io/odh-platform/pkg/spi"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *PlatformRoutingController) createRoutingResources(ctx context.Context, target *unstructured.Unstructured) error {
	if IsMarkedForDeletion(target) {
		return nil
	}

	exportModes := extractExportModes(target)

	if len(exportModes) == 0 {
		r.log.Info("No export mode found for target")

		return nil
	}

	r.log.Info("Reconciling resources for target", "target", target)

	exportedServices, errSvcGet := getExportedServices(ctx, r.Client, target)
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

func (r *PlatformRoutingController) exportService(ctx context.Context, target *unstructured.Unstructured, exportedSvc *corev1.Service, domain string) error {
	exportModes := extractExportModes(target)
	if len(exportModes) == 0 {
		return nil
	}

	templateData := spi.RoutingTemplateData{
		PlatformRoutingConfiguration: r.config,
		PublicServiceName:            exportedSvc.GetName() + "-" + exportedSvc.GetNamespace(),
		ServiceName:                  exportedSvc.GetName(),
		ServiceNamespace:             exportedSvc.GetNamespace(),
		ServiceTargetPort:            exportedSvc.Spec.Ports[0].TargetPort.String(),
		Domain:                       domain,
	}

	labelsForCreatedResources := []metadata.Options{
		// TODO(mvp): add standard labels
		metadata.WithOwnerLabels(target), // To establish ownership for watched component
		metadata.WithLabels(metadata.Labels.AppManagedBy, "odh-routing-controller"),
	}

	targetKey := client.ObjectKeyFromObject(target)

	for _, exportMode := range exportModes {
		resources, err := r.templateLoader.Load(ctx, exportMode, targetKey, templateData)
		if err != nil {
			return fmt.Errorf("could not load templates for type %s: %w", exportMode, err)
		}

		labelsForCreatedResources = append(labelsForCreatedResources, metadata.WithLabels(metadata.Labels.ExportType, string(exportMode)))
		if errApply := cluster.Apply(ctx, r.Client, resources, labelsForCreatedResources...); errApply != nil {
			return fmt.Errorf("could not apply routing resources for type %s: %w", exportMode, errApply)
		}
	}

	return propagateHostsToWatchedCR(target, templateData)
}

func propagateHostsToWatchedCR(target *unstructured.Unstructured, data spi.RoutingTemplateData) error {
	exportModes := extractExportModes(target)
	if len(exportModes) == 0 {
		return nil
	}

	var metaOptions []metadata.Options

	// TODO(mvp): put the logic of creating host names into a single place
	for _, exportMode := range exportModes {
		switch exportMode {
		case spi.ExternalRoute:
			externalAddress := metadata.WithAnnotations(metadata.Annotations.RoutingAddressesExternal, fmt.Sprintf("%s-%s.%s", data.ServiceName, data.ServiceNamespace, data.Domain))
			metaOptions = append(metaOptions, externalAddress)
		case spi.PublicRoute:
			publicAddresses := metadata.WithAnnotations(
				metadata.Annotations.RoutingAddressesPublic,
				fmt.Sprintf("%[1]s.%[2]s;%[1]s.%[2]s.svc;%[1]s.%[2]s.svc.cluster.local", data.PublicServiceName, data.GatewayNamespace),
			)
			metaOptions = append(metaOptions, publicAddresses)
		}
	}

	if errApply := metadata.ApplyMetaOptions(target, metaOptions...); errApply != nil {
		return fmt.Errorf("could not propagate hosts back to target %s/%s : %w", target.GetObjectKind().GroupVersionKind().Kind, target.GetName(), errApply)
	}

	return nil
}

func extractExportModes(target *unstructured.Unstructured) []spi.RouteType {
	exportModes, exportModeFound := target.GetAnnotations()[metadata.Annotations.RoutingExportMode]
	if !exportModeFound {
		return nil
	}

	exportModesSplit := strings.Split(exportModes, ";")
	routeTypes := make([]spi.RouteType, len(exportModesSplit))

	for i, exportMode := range exportModesSplit {
		routeTypes[i] = spi.RouteType(exportMode)
	}

	return routeTypes
}
