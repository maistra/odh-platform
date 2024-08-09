package routing_test

import (
	"context"
	"errors"

	. "github.com/onsi/gomega"
	"github.com/opendatahub-io/odh-platform/pkg/cluster"
	"github.com/opendatahub-io/odh-platform/pkg/metadata"
	"github.com/opendatahub-io/odh-platform/test"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func getClusterDomain(ctx context.Context) string {
	var domain string

	Eventually(func(ctx context.Context) error {
		var err error
		domain, err = cluster.GetDomain(ctx, envTest.Client)

		return err
	}).WithContext(ctx).
		WithTimeout(test.DefaultTimeout).
		WithPolling(test.DefaultPolling).
		Should(Succeed())

	return domain
}

func exportCustomResource(ctx context.Context, exportedComponent *unstructured.Unstructured, mode string) {
	// routing.opendatahub.io/export-mode: "public;external"
	exposeExternally := metadata.WithAnnotations(metadata.Annotations.RoutingExportMode, mode)
	_, errExportCR := controllerutil.CreateOrUpdate(
		ctx, envTest.Client,
		exportedComponent,
		func() error {
			return metadata.ApplyMetaOptions(exportedComponent, exposeExternally)
		})
	Expect(errExportCR).ToNot(HaveOccurred())
}

func addRoutingRequirementsToSvc(ctx context.Context, exportedSvc *corev1.Service, owningComponent *unstructured.Unstructured) {
	// routing.opendatahub.io/exported: "true"
	exportAnnotation := metadata.WithLabels(metadata.Labels.RoutingExported, "true")
	// platform.opendatahub.io/owner-name: test-component
	// platform.opendatahub.io/owner-kind: Component
	ownerLabels := metadata.WithLabels(
		metadata.Labels.OwnerName, owningComponent.GetName(),
		metadata.Labels.OwnerKind, owningComponent.GetKind(),
	)

	// Service created by the component need to have these metadata added, i.e. by its controller
	_, errExportSvc := controllerutil.CreateOrUpdate(ctx, envTest.Client, exportedSvc, func() error {
		return metadata.ApplyMetaOptions(exportedSvc, exportAnnotation, ownerLabels)
	})
	Expect(errExportSvc).ToNot(HaveOccurred())
}

func ensureFinalizersSet(ctx context.Context, owningComponent *unstructured.Unstructured) *unstructured.Unstructured {
	// Re-fetch the component from the cluster to get the latest version (ensuring finalizers are set)
	Eventually(func() error {
		errGetComponent := envTest.Client.Get(ctx, client.ObjectKey{
			Namespace: owningComponent.GetNamespace(),
			Name:      owningComponent.GetName(),
		}, owningComponent)

		if errGetComponent != nil {
			return errGetComponent
		}

		if len(owningComponent.GetFinalizers()) == 0 {
			return errors.New("finalizers are not yet set")
		}

		return nil
	}).WithContext(ctx).
		WithTimeout(test.DefaultTimeout).
		WithPolling(test.DefaultPolling).
		Should(Succeed())

	return owningComponent
}
