package test

import (
	"context"
	"errors"

	. "github.com/onsi/gomega"
	"github.com/opendatahub-io/odh-platform/pkg/cluster"
	"github.com/opendatahub-io/odh-platform/pkg/metadata"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func GetClusterDomain(ctx context.Context, cli client.Client) string {
	var domain string

	Eventually(func(ctx context.Context) error {
		var err error
		domain, err = cluster.GetDomain(ctx, cli)

		return err
	}).WithContext(ctx).
		WithTimeout(DefaultTimeout).
		WithPolling(DefaultPolling).
		Should(Succeed())

	return domain
}

func AddRoutingRequirements(ctx context.Context, component *unstructured.Unstructured, svc *corev1.Service, mode string, cli client.Client) *unstructured.Unstructured {
	// routing.opendatahub.io/exported: "true"
	exportAnnotation := metadata.WithLabels(metadata.Labels.RoutingExported, "true")
	// platform.opendatahub.io/owner-name: test-component
	// platform.opendatahub.io/owner-kind: Component
	ownerLabels := metadata.WithOwnerLabels(component)

	// Service created by the component need to have these metadata added, i.e. by its controller
	_, errExportSvc := controllerutil.CreateOrUpdate(ctx, cli, svc, func() error {
		return metadata.ApplyMetaOptions(svc, exportAnnotation, ownerLabels)
	})
	Expect(errExportSvc).ToNot(HaveOccurred())

	// Re-fetch the component from the cluster to get the latest version (ensuring finalizers are set)
	Eventually(func() error {
		errGetComponent := cli.Get(ctx, client.ObjectKey{
			Namespace: component.GetNamespace(),
			Name:      component.GetName(),
		}, component)

		if errGetComponent != nil {
			return errGetComponent
		}

		if len(component.GetFinalizers()) == 0 {
			return errors.New("finalizers are not yet set")
		}

		return nil
	}).WithContext(ctx).
		WithTimeout(DefaultTimeout).
		WithPolling(DefaultPolling).
		Should(Succeed())

	// routing.opendatahub.io/export-mode: inputted mode
	exposeExternally := metadata.WithAnnotations(metadata.Annotations.RoutingExportMode, mode)
	_, errExportCR := controllerutil.CreateOrUpdate(ctx, cli, component, func() error {
		return metadata.ApplyMetaOptions(component, exposeExternally)
	})
	Expect(errExportCR).ToNot(HaveOccurred())

	return component
}
