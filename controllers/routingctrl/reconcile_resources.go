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
	"github.com/opendatahub-io/odh-platform/pkg/routing"
	"github.com/opendatahub-io/odh-platform/pkg/unstruct"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *Controller) createRoutingResources(ctx context.Context, target *unstructured.Unstructured) error {
	exportModes := r.extractExportModes(target)

	if len(exportModes) == 0 {
		r.log.Info("No export mode found for target")

		return r.propagateHostsToWatchedCR(ctx, target, nil, nil)
	}

	r.log.Info("Reconciling resources for target", "target", target)

	renderedSelectors, errLables := config.ResolveSelectors(r.component.ServiceSelector, target)
	if errLables != nil {
		return fmt.Errorf("could not render labels for ServiceSelector %v: %w", r.component.ServiceSelector, errLables)
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

	externalHosts := []string{}
	publicHosts := []string{}

	// To establish ownership for watched component
	ownershipLabels := append(labels.AsOwner(target), labels.AppManagedBy("odh-routing-controller"))

	for _, exportedSvcPort := range exportedSvc.Spec.Ports {
		templateData := routing.NewExposedServiceConfig(exportedSvc, exportedSvcPort, r.config, domain)

		for _, exportMode := range exportModes {
			resources, err := r.templateLoader.Load(templateData, exportMode)
			if err != nil {
				return fmt.Errorf("could not load templates for type %s: %w", exportMode, err)
			}

			ownershipLabels = append(ownershipLabels, labels.ExportType(exportMode))
			if errApply := unstruct.Apply(ctx, r.Client, resources, ownershipLabels...); errApply != nil {
				return fmt.Errorf("could not apply routing resources for type %s: %w", exportMode, errApply)
			}

			switch exportMode {
			case routing.ExternalRoute:
				externalHosts = append(externalHosts, templateData.ExternalHost())
			case routing.PublicRoute:
				publicHosts = append(publicHosts, templateData.PublicHosts()...)
			}
		}
	}

	return r.propagateHostsToWatchedCR(ctx, target, publicHosts, externalHosts)
}

func (r *Controller) propagateHostsToWatchedCR(ctx context.Context, target *unstructured.Unstructured, publicHosts, externalHosts []string) error {
	err := unstruct.PatchWithRetry(ctx, r.Client, target, func() error {
		// Always remove the annotations first
		annotations.Remove(annotations.RoutingAddressesExternal(""))(target)
		annotations.Remove(annotations.RoutingAddressesPublic(""))(target)

		var metaOptions []metadata.Option

		if len(publicHosts) > 0 {
			metaOptions = append(metaOptions, annotations.RoutingAddressesPublic(strings.Join(publicHosts, ";")))
		}

		if len(externalHosts) > 0 {
			metaOptions = append(metaOptions, annotations.RoutingAddressesExternal(strings.Join(externalHosts, ";")))
		}

		metadata.ApplyMetaOptions(target, metaOptions...)

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to propagate hosts to watched CR %s/%s: %w", target.GetNamespace(), target.GetName(), err)
	}

	return nil
}

func (r *Controller) ensureResourceHasFinalizer(ctx context.Context, target *unstructured.Unstructured) error {
	if !controllerutil.ContainsFinalizer(target, finalizerName) {
		if err := unstruct.PatchWithRetry(ctx, r.Client, target, func() error {
			controllerutil.AddFinalizer(target, finalizerName)

			return nil
		}); err != nil {
			return fmt.Errorf("failed to patch finalizer to %s (in %s): %w",
				target.GroupVersionKind().String(), target.GetNamespace(), err)
		}
	}

	return nil
}

// extractExportModes retrieves the enabled export modes from the target's annotations.
func (r *Controller) extractExportModes(target *unstructured.Unstructured) []routing.RouteType {
	targetAnnotations := target.GetAnnotations()
	if targetAnnotations == nil {
		return nil
	}

	validRouteTypes := make([]routing.RouteType, 0)

	for key, value := range targetAnnotations {
		if value == "true" {
			routeType, valid := routing.IsValidRouteType(key)
			if valid {
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
