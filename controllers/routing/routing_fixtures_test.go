package routing_test

import (
	"context"

	. "github.com/onsi/gomega"
	"github.com/opendatahub-io/odh-platform/pkg/cluster"
	"github.com/opendatahub-io/odh-platform/pkg/metadata"
	"github.com/opendatahub-io/odh-platform/pkg/metadata/annotations"
	"github.com/opendatahub-io/odh-platform/pkg/metadata/labels"
	"github.com/opendatahub-io/odh-platform/test"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func getClusterDomain(ctx context.Context, cli client.Client) string {
	var domain string

	Eventually(func(ctx context.Context) error {
		var err error
		domain, err = cluster.GetDomain(ctx, cli)

		return err
	}).WithContext(ctx).
		WithTimeout(test.DefaultTimeout).
		WithPolling(test.DefaultPolling).
		Should(Succeed())

	return domain
}

// addRoutingRequirementsToSvc adds routing-related metadata to the Service being exported.
// It adds the "routing.opendatahub.io/exported" label to indicate that the service is exported,
// and it also sets labels for the owner component's name and kind, using
// "platform.opendatahub.io/owner-name" and "platform.opendatahub.io/owner-kind" respectively.
func addRoutingRequirementsToSvc(ctx context.Context, exportedSvc *corev1.Service, owningComponent *unstructured.Unstructured) {
	exportedLabel := labels.RoutingExported("true")
	ownerName := labels.OwnerName(owningComponent.GetName())
	ownerKind := labels.OwnerKind(owningComponent.GetObjectKind().GroupVersionKind().Kind)

	_, errExportSvc := controllerutil.CreateOrUpdate(ctx, envTest.Client, exportedSvc, func() error {
		metadata.ApplyMetaOptions(exportedSvc, exportedLabel, ownerName, ownerKind)

		return nil
	})
	Expect(errExportSvc).ToNot(HaveOccurred())
}

func createComponentRequiringPlatformRouting(ctx context.Context, componentName, mode, appNs string) (*unstructured.Unstructured, error) {
	component, errCreate := test.CreateUnstructured(componentResource(componentName, appNs))
	Expect(errCreate).ToNot(HaveOccurred())

	// set component's "routing.opendatahub.io/export-mode" annotation to the specified mode.
	metadata.ApplyMetaOptions(component, annotations.RoutingExportMode(mode))

	return component, envTest.Client.Create(ctx, component)
}
