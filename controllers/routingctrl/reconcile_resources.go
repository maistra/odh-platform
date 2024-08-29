package routingctrl

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/opendatahub-io/odh-platform/pkg/cluster"
	"github.com/opendatahub-io/odh-platform/pkg/config"
	"github.com/opendatahub-io/odh-platform/pkg/metadata"
	"github.com/opendatahub-io/odh-platform/pkg/metadata/annotations"
	"github.com/opendatahub-io/odh-platform/pkg/metadata/labels"
	"github.com/opendatahub-io/odh-platform/pkg/spi"
	"github.com/opendatahub-io/odh-platform/pkg/unstruct"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *Controller) createRoutingResources(ctx context.Context, target *unstructured.Unstructured) error {
	exportModes := r.extractExportModes(target)

	if len(exportModes) == 0 {
		r.log.Info("No export mode found for target")
		metadata.ApplyMetaOptions(target,
			annotations.Remove(annotations.RoutingAddressesExternal("")),
			annotations.Remove(annotations.RoutingAddressesPublic("")),
		)

		return nil
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
	exportModes := r.extractExportModes(target)

	templateData := spi.NewRoutingData(r.config, exportedSvc, domain)

	// To establish ownership for watched component
	ownershipLabels := append(labels.AsOwner(target), labels.AppManagedBy("odh-routing-controller"))

	for _, exportMode := range exportModes {
		resources, err := r.templateLoader.Load(templateData, exportMode)
		if err != nil {
			return fmt.Errorf("could not load templates for type %s: %w", exportMode, err)
		}

		ownershipLabels = append(ownershipLabels, labels.ExportType(exportMode))
		if errApply := unstruct.Apply(ctx, r.Client, resources, ownershipLabels...); errApply != nil {
			return fmt.Errorf("could not apply routing resources for type %s: %w", exportMode, errApply)
		}
	}

	return r.propagateHostsToWatchedCR(target, templateData)
}

func (r *Controller) propagateHostsToWatchedCR(target *unstructured.Unstructured, data *spi.RoutingData) error {
	exportModes := r.extractExportModes(target)

	// Remove all existing routing addresses
	metaOptions := []metadata.Option{
		annotations.Remove(annotations.RoutingAddressesExternal("")),
		annotations.Remove(annotations.RoutingAddressesPublic("")),
	}

	// TODO(mvp): put the logic of creating host names into a single place
	for _, exportMode := range exportModes {
		switch exportMode {
		case spi.ExternalRoute:
			externalAddress := annotations.RoutingAddressesExternal(fmt.Sprintf("%s-%s.%s", data.ServiceName, data.ServiceNamespace, data.Domain))
			metaOptions = append(metaOptions, externalAddress)
		case spi.PublicRoute:
			publicAddresses := annotations.RoutingAddressesPublic(fmt.Sprintf("%[1]s.%[2]s;%[1]s.%[2]s.svc;%[1]s.%[2]s.svc.cluster.local", data.PublicServiceName, data.GatewayNamespace))
			metaOptions = append(metaOptions, publicAddresses)
		}
	}

	metadata.ApplyMetaOptions(target, metaOptions...)

	return nil
}

func (r *Controller) ensureResourceHasFinalizer(ctx context.Context, target *unstructured.Unstructured) error {
	if controllerutil.AddFinalizer(target, finalizerName) {
		if err := unstruct.Patch(ctx, r.Client, target); err != nil {
			return fmt.Errorf("failed to patch finalizer to resource %s/%s: %w", target.GetNamespace(), target.GetName(), err)
		}
	}

	return nil
}

// extractExportModes retrieves the enabled export modes from the target's annotations.
func (r *Controller) extractExportModes(target *unstructured.Unstructured) []spi.RouteType {
	targetAnnotations := target.GetAnnotations()
	if targetAnnotations == nil {
		return nil
	}

	validRouteTypes := make([]spi.RouteType, 0)

	for key, value := range targetAnnotations {
		if strings.HasPrefix(key, annotations.RoutingExportModePrefix) && value == "true" {
			routeType := spi.RouteType(strings.TrimPrefix(key, annotations.RoutingExportModePrefix))
			if spi.IsValidRouteType(routeType) {
				validRouteTypes = append(validRouteTypes, routeType)
			} else {
				r.log.Info("Invalid route type found",
					"invalidRouteType", routeType,
					"resourceName", target.GetName(),
					"resourceNamespace", target.GetNamespace())
			}
		}
	}

	return validRouteTypes
}
