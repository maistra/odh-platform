package routing

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/opendatahub-io/odh-platform/pkg/cluster"
	"github.com/opendatahub-io/odh-platform/pkg/metadata"
	"github.com/opendatahub-io/odh-platform/pkg/spi"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

func (r *PlatformRoutingReconciler) reconcileResources(ctx context.Context, target *unstructured.Unstructured) error {
	exportModes, exportModeFound := extractExportModes(target)
	// TODO shouldn't we make it a predicate for ctrl watch instead?
	if !exportModeFound {
		return nil
	}

	r.log.Info("Reconciling resources for target", "target", target)

	exportedSvc, errSvcGet := getExportedService(ctx, r.Client, target)
	if errSvcGet != nil {
		if errors.Is(errSvcGet, &NoExportedServicesError{}) {
			r.log.Info("no exported service found for target", "target", target)

			return nil
		}

		return errSvcGet
	}

	domain, errDomain := cluster.GetDomain(ctx, r.Client)
	if errDomain != nil {
		return fmt.Errorf("could not get domain: %w", errDomain)
	}

	templateData := spi.RoutingTemplateData{
		PublicServiceName: exportedSvc.GetName() + "-" + exportedSvc.GetNamespace(),
		ServiceName:       exportedSvc.GetName(),
		ServiceNamespace:  exportedSvc.GetNamespace(),
		ServiceTargetPort: exportedSvc.Spec.Ports[0].TargetPort.String(),
		Domain:            domain,
		// TODO: compose instead
		IngressSelectorLabel: r.config.IngressSelectorLabel,
		IngressSelectorValue: r.config.IngressSelectorValue,
		IngressService:       r.config.IngressService,
		GatewayNamespace:     r.config.GatewayNamespace,
	}

	withOwnershipLabels := ownershipLabels(target)

	targetKey := k8stypes.NamespacedName{Namespace: target.GetNamespace(), Name: target.GetName()}

	for _, exportMode := range exportModes {
		resources, err := r.templateLoader.Load(ctx, exportMode, targetKey, templateData)
		if err != nil {
			return fmt.Errorf("could not load templates for type %s: %w", exportMode, err)
		}

		if errApply := cluster.Apply(ctx, r.Client, resources, withOwnershipLabels...); errApply != nil {
			return fmt.Errorf("could not apply routing resources for type %s: %w", exportMode, errApply)
		}
	}

	return propagateHostsToWatchedCR(target, templateData)
}

func propagateHostsToWatchedCR(target *unstructured.Unstructured, data spi.RoutingTemplateData) error {
	var metaOptions []metadata.Options

	exportModes, found := extractExportModes(target)
	if !found {
		return fmt.Errorf("could not extract export modes from target %s", target.GetName())
	}

	// TODO(mvp) : put the logic of creating host names into a single place
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

func ownershipLabels(target *unstructured.Unstructured) []metadata.Options {
	return []metadata.Options{
		metadata.WithOwnerLabels(target),
		metadata.WithLabels(metadata.Labels.AppManagedBy, "odh-routing-controller"),
	}
}

func extractExportModes(target *unstructured.Unstructured) ([]spi.RouteType, bool) {
	exportModes, exportModeFound := target.GetAnnotations()[metadata.Annotations.RoutingExportMode]
	if !exportModeFound {
		return nil, false
	}

	exportModesSplit := strings.Split(exportModes, ";")
	routeTypes := make([]spi.RouteType, len(exportModesSplit))

	for i, exportMode := range exportModesSplit {
		routeTypes[i] = spi.RouteType(exportMode)
	}

	return routeTypes, true
}
