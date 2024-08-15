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
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *PlatformRoutingController) reconcileResources(ctx context.Context, target *unstructured.Unstructured) error {
	// TODO shouldn't we make it a predicate for ctrl watch instead?
	_, exportModeFound := extractExportModes(target)
	if !exportModeFound {
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
	exportModes, found := extractExportModes(target)
	if !found {
		return fmt.Errorf("could not extract export modes from target %s", target.GetName())
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

	targetKey := k8stypes.NamespacedName{Namespace: target.GetNamespace(), Name: target.GetName()}

	for _, exportMode := range exportModes {
		resources, err := r.templateLoader.Load(ctx, exportMode, targetKey, templateData)
		if err != nil {
			return fmt.Errorf("could not load templates for type %s: %w", exportMode, err)
		}

		if errApply := cluster.Apply(ctx, r.Client, resources, labelsForCreatedResources...); errApply != nil {
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

func (r *PlatformRoutingController) patchResourceMetadata(ctx context.Context, req ctrl.Request, sourceRes *unstructured.Unstructured) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		currentRes := &unstructured.Unstructured{}
		currentRes.SetGroupVersionKind(sourceRes.GroupVersionKind())

		if err := r.Client.Get(ctx, req.NamespacedName, currentRes); err != nil {
			return fmt.Errorf("failed re-fetching resource: %w", err)
		}

		patch := client.MergeFrom(currentRes)
		if errPatch := r.Client.Patch(ctx, sourceRes, patch); errPatch != nil {
			return fmt.Errorf("failed to patch resource with updated annotations: %w", errPatch)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to patch resource metadata with retry: %w", err)
	}

	return nil
}

func (r *PlatformRoutingController) addResourceFinalizer(ctx context.Context, req ctrl.Request, sourceRes *unstructured.Unstructured) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		currentRes := &unstructured.Unstructured{}
		currentRes.SetGroupVersionKind(sourceRes.GroupVersionKind())

		if err := r.Client.Get(ctx, req.NamespacedName, sourceRes); err != nil {
			return fmt.Errorf("failed re-fetching resource: %w", err)
		}

		if controllerutil.AddFinalizer(sourceRes, finalizerName) {
			patch := client.MergeFrom(currentRes)
			if err := r.Client.Patch(ctx, sourceRes, patch); err != nil {
				return fmt.Errorf("failed to patch resource with finalizer: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed adding finalizer with retry: %w", err)
	}

	return nil
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
