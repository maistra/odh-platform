package routingctrl_test

import (
	"context"

	. "github.com/onsi/gomega"
	"github.com/opendatahub-io/odh-platform/pkg/cluster"
	"github.com/opendatahub-io/odh-platform/pkg/metadata"
	"github.com/opendatahub-io/odh-platform/pkg/metadata/annotations"
	"github.com/opendatahub-io/odh-platform/pkg/metadata/labels"
	"github.com/opendatahub-io/odh-platform/test"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// addRoutingRequirementsToSvc adds routing-related metadata to the Service being exported to match the
// serviceSelector defined in the suite_test.
func addRoutingRequirementsToSvc(ctx context.Context, exportedSvc *corev1.Service, owningComponent *unstructured.Unstructured) {
	ownerName := labels.OwnerName(owningComponent.GetName())
	ownerKind := labels.OwnerKind(owningComponent.GetObjectKind().GroupVersionKind().Kind)

	_, errExportSvc := controllerutil.CreateOrUpdate(ctx, envTest.Client, exportedSvc, func() error {
		metadata.ApplyMetaOptions(exportedSvc, ownerName, ownerKind)

		return nil
	})

	Expect(errExportSvc).ToNot(HaveOccurred())
}

// createComponentRequiringPlatformRouting creates a new component with the specified routing modes.
func createComponentRequiringPlatformRouting(ctx context.Context, componentName, appNs string, modes ...annotations.RoutingExportMode) (*unstructured.Unstructured, error) {
	component, errCreate := test.CreateUnstructured(componentResource(componentName, appNs))
	Expect(errCreate).ToNot(HaveOccurred())

	for _, mode := range modes {
		metadata.ApplyMetaOptions(component, mode)
	}

	return component, envTest.Client.Create(ctx, component)
}

type exportModeAction struct {
	mode  string
	value string
}

var (
	enablePublic           = exportModeAction{mode: annotations.PublicMode().Key(), value: "true"}
	disableExternal        = exportModeAction{mode: annotations.ExternalMode().Key(), value: "false"}
	removeExternal         = exportModeAction{mode: annotations.ExternalMode().Key(), value: ""}
	removePublic           = exportModeAction{mode: annotations.PublicMode().Key(), value: ""}
	enableNonSupportedMode = exportModeAction{
		mode:  annotations.RoutingExportModePrefix + "notsupported",
		value: "true",
	}
)

func (m exportModeAction) ApplyToMeta(obj metav1.Object) {
	annos := obj.GetAnnotations()
	if annos == nil {
		annos = make(map[string]string)
	}

	key := m.mode

	if m.value == "" {
		delete(annos, key)
	} else {
		annos[key] = m.value
	}

	obj.SetAnnotations(annos)
}

func setExportModes(ctx context.Context, component *unstructured.Unstructured, actions ...exportModeAction) {
	errGetComponent := envTest.Client.Get(ctx, client.ObjectKey{
		Namespace: component.GetNamespace(),
		Name:      component.GetName(),
	}, component)
	Expect(errGetComponent).ToNot(HaveOccurred())

	for _, action := range actions {
		metadata.ApplyMetaOptions(component, action)
	}

	Expect(envTest.Client.Update(ctx, component)).To(Succeed())
}
