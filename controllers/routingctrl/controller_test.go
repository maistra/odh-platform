package routingctrl_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/opendatahub-io/odh-platform/pkg/metadata/annotations"
	"github.com/opendatahub-io/odh-platform/test"
	. "github.com/opendatahub-io/odh-platform/test/matchers"
	openshiftroutev1 "github.com/openshift/api/route/v1"
	istionetworkingv1beta1 "istio.io/api/networking/v1beta1"
	"istio.io/client-go/pkg/apis/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const watchedCR = `
apiVersion: opendatahub.io/v1
kind: Component
metadata:
  name: %[1]s
  namespace: %[2]s
spec:
  name: %[1]s
`

var domain string

var _ = Describe("Platform routing setup for the component", test.EnvTest(), func() {

	var (
		routerNs   *corev1.Namespace
		appNs      *corev1.Namespace
		deployment *appsv1.Deployment
		svc        *corev1.Service

		toRemove []client.Object
	)

	BeforeEach(func(ctx context.Context) {
		routerNs = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: routingConfiguration.GatewayNamespace,
			},
		}
		Expect(envTest.Client.Create(ctx, routerNs)).To(Succeed())

		base := "app-ns"
		testNamespaceName := fmt.Sprintf("%s-%s", base, utilrand.String(7))

		appNs = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespaceName,
			},
		}
		Expect(envTest.Client.Create(ctx, appNs)).To(Succeed())

		deployment, svc = simpleSvcDeployment(ctx, appNs.Name, "mesh-service-name")

		toRemove = []client.Object{routerNs, deployment, svc}

		if !envTest.UsingExistingCluster() {
			ingressConfig, errIngress := test.DefaultIngressControllerConfig(ctx, envTest.Client)
			Expect(errIngress).ToNot(HaveOccurred())
			toRemove = append(toRemove, ingressConfig)
		}

		domain = getClusterDomain(ctx, envTest.Client)
	})

	AfterEach(func(_ context.Context) {
		envTest.DeleteAll(toRemove...)
	})

	When("watched component requests to expose service externally to the cluster", func() {

		It("should have external routing resources created", func(ctx context.Context) {
			// given
			// required annotation for watched custom resource:
			// routing.opendatahub.io/export-mode-external: "true"
			component, createErr := createComponentRequiringPlatformRouting(ctx, "exported-test-component", appNs.Name, annotations.ExternalMode())
			Expect(createErr).ToNot(HaveOccurred())
			toRemove = append(toRemove, component)

			// required labels for the exported service:
			// 	platform.opendatahub.io/owner-name: test-component
			// 	platform.opendatahub.io/owner-kind: Component
			addRoutingRequirementsToSvc(ctx, svc, component)

			// then
			externalResourcesShouldExist(ctx, svc)
		})

		It("should have new hosts propagated back to watched resource", func(ctx context.Context) {
			// given
			// required annotation for watched custom resource:
			// routing.opendatahub.io/export-mode-external: "true"
			component, createErr := createComponentRequiringPlatformRouting(ctx, "exported-test-component", appNs.Name, annotations.ExternalMode())
			Expect(createErr).ToNot(HaveOccurred())
			toRemove = append(toRemove, component)

			// required labels for the exported service:
			// 	routing.opendatahub.io/exported: "true"
			// 	platform.opendatahub.io/owner-name: test-component
			// 	platform.opendatahub.io/owner-kind: Component
			addRoutingRequirementsToSvc(ctx, svc, component)

			// then
			Eventually(func(g Gomega, ctx context.Context) error {
				updatedComponent := component.DeepCopy()
				if errGet := envTest.Get(ctx, client.ObjectKeyFromObject(updatedComponent), updatedComponent); errGet != nil {
					return errGet
				}

				g.Expect(updatedComponent.GetAnnotations()).ToNot(HaveKey(
					annotations.RoutingAddressesPublic("").Key(),
				), "public services are not expected to be defined in this mode")

				externalAddressesAnnotation := annotations.RoutingAddressesExternal(
					fmt.Sprintf("%[1]s-http-%[2]s.%[3]s"+";"+"%[1]s-grpc-%[2]s.%[3]s", svc.Name, svc.Namespace, domain))

				g.Expect(updatedComponent.GetAnnotations()).To(HaveKeyWithValue(
					externalAddressesAnnotation.Key(), externalAddressesAnnotation.Value(),
				))

				return nil
			}).
				WithContext(ctx).
				WithTimeout(test.DefaultTimeout).
				WithPolling(test.DefaultPolling).
				Should(Succeed())
		})

	})

	When("watched component requests to expose service locally (outside of service mesh) to the cluster", func() {

		It("should have routing resources for out-of-mesh access created", func(ctx context.Context) {
			// given
			// required annotation for watched custom resource:
			// routing.opendatahub.io/export-mode-public: "true"
			component, createErr := createComponentRequiringPlatformRouting(ctx, "public-test-component", appNs.Name, annotations.PublicMode())
			Expect(createErr).ToNot(HaveOccurred())
			toRemove = append(toRemove, component)

			// required labels for the exported service:
			// 	routing.opendatahub.io/exported: "true"
			// 	platform.opendatahub.io/owner-name: test-component
			// 	platform.opendatahub.io/owner-kind: Component
			addRoutingRequirementsToSvc(ctx, svc, component)

			// then
			publicResourcesShouldExist(ctx, svc)

		})

		It("should have new hosts propagated back to watched resource by the controller", func(ctx context.Context) {
			// given
			// required annotation for watched custom resource:
			// routing.opendatahub.io/export-mode-public: "true"
			component, createErr := createComponentRequiringPlatformRouting(ctx, "public-test-component", appNs.Name, annotations.PublicMode())
			Expect(createErr).ToNot(HaveOccurred())
			toRemove = append(toRemove, component)

			// required labels for the exported service:
			// 	routing.opendatahub.io/exported: "true"
			// 	platform.opendatahub.io/owner-name: test-component
			// 	platform.opendatahub.io/owner-kind: Component
			addRoutingRequirementsToSvc(ctx, svc, component)

			// then
			Eventually(func(g Gomega, ctx context.Context) error {
				updatedComponent := component.DeepCopy()
				if errGet := envTest.Get(ctx, client.ObjectKeyFromObject(updatedComponent), updatedComponent); errGet != nil {
					return errGet
				}

				g.Expect(updatedComponent.GetAnnotations()).ToNot(
					HaveKey(
						annotations.RoutingAddressesExternal("").Key(),
					), "public services are not expected to be defined in this mode")

				publicAddressAnnotation := annotations.RoutingAddressesPublic(
					fmt.Sprintf("%[1]s-http-%[2]s.%[3]s;%[1]s-http-%[2]s.%[3]s.svc;%[1]s-http-%[2]s.%[3]s.svc.cluster.local;"+
						"%[1]s-grpc-%[2]s.%[3]s;%[1]s-grpc-%[2]s.%[3]s.svc;%[1]s-grpc-%[2]s.%[3]s.svc.cluster.local",
						svc.Name, svc.Namespace, routingConfiguration.GatewayNamespace),
				)

				g.Expect(updatedComponent.GetAnnotations()).To(
					HaveKeyWithValue(publicAddressAnnotation.Key(), publicAddressAnnotation.Value()),
				)

				return nil
			}).
				WithContext(ctx).
				WithTimeout(test.DefaultTimeout).
				WithPolling(test.DefaultPolling).
				Should(Succeed())
		})

	})

	When("component requests to expose service both locally and externally to the cluster", func() {

		It("should have both external and cluster-local resources created", func(ctx context.Context) {
			// given
			// required annotation for watched custom resource:
			// routing.opendatahub.io/export-mode-external: "true"
			// routing.opendatahub.io/export-mode-public: "true"
			component, createErr := createComponentRequiringPlatformRouting(ctx, "public-and-external-test-component",
				appNs.Name, annotations.ExternalMode(), annotations.PublicMode())
			Expect(createErr).ToNot(HaveOccurred())
			toRemove = append(toRemove, component)

			// required labels for the exported service:
			// 	routing.opendatahub.io/exported: "true"
			// 	platform.opendatahub.io/owner-name: test-component
			// 	platform.opendatahub.io/owner-kind: Component
			addRoutingRequirementsToSvc(ctx, svc, component)

			// then
			externalResourcesShouldExist(ctx, svc)
			publicResourcesShouldExist(ctx, svc)

			Eventually(func(g Gomega, ctx context.Context) error {
				updatedComponent := component.DeepCopy()
				if errGet := envTest.Get(ctx, client.ObjectKeyFromObject(updatedComponent), updatedComponent); errGet != nil {
					return errGet
				}

				externalAddressAnnotation := annotations.RoutingAddressesExternal(
					fmt.Sprintf("%[1]s-http-%[2]s.%[3]s"+";"+"%[1]s-grpc-%[2]s.%[3]s", svc.Name, svc.Namespace, domain))

				publicAddrAnnotation := annotations.RoutingAddressesPublic(
					fmt.Sprintf("%[1]s-http-%[2]s.%[3]s;%[1]s-http-%[2]s.%[3]s.svc;%[1]s-http-%[2]s.%[3]s.svc.cluster.local;"+
						"%[1]s-grpc-%[2]s.%[3]s;%[1]s-grpc-%[2]s.%[3]s.svc;%[1]s-grpc-%[2]s.%[3]s.svc.cluster.local",
						svc.Name, svc.Namespace, routingConfiguration.GatewayNamespace,
					),
				)

				g.Expect(updatedComponent.GetAnnotations()).To(
					And(
						HaveKeyWithValue(
							externalAddressAnnotation.Key(), externalAddressAnnotation.Value(),
						),
						HaveKeyWithValue(
							publicAddrAnnotation.Key(), publicAddrAnnotation.Value(),
						),
					),
				)

				return nil
			}).
				WithContext(ctx).
				WithTimeout(test.DefaultTimeout).
				WithPolling(test.DefaultPolling).
				Should(Succeed())

		})

	})

	When("component is deleted all routing resources should be removed", func() {

		It("should remove the routing resources when both public;external", func(ctx context.Context) {
			// given
			// required annotation for watched custom resource:
			// routing.opendatahub.io/export-mode-external: "true"
			// routing.opendatahub.io/export-mode-public: "true"
			component, createErr := createComponentRequiringPlatformRouting(ctx, "public-and-external-test-component",
				appNs.Name, annotations.ExternalMode(), annotations.PublicMode())
			Expect(createErr).ToNot(HaveOccurred())
			toRemove = append(toRemove, component)

			// required labels for the exported service:
			// 	routing.opendatahub.io/exported: "true"
			// 	platform.opendatahub.io/owner-name: test-component
			// 	platform.opendatahub.io/owner-kind: Component
			addRoutingRequirementsToSvc(ctx, svc, component)

			// then
			externalResourcesShouldExist(ctx, svc)
			publicResourcesShouldExist(ctx, svc)

			// when
			By("deleting the component", func() {
				// Re-fetch the component from the cluster to get the latest version
				errGetComponent := envTest.Client.Get(ctx, client.ObjectKey{
					Namespace: component.GetNamespace(),
					Name:      component.GetName(),
				}, component)
				Expect(errGetComponent).ToNot(HaveOccurred())

				Expect(envTest.Client.Delete(ctx, component)).To(Succeed())
			})

			// then
			externalResourcesShouldNotExist(ctx, svc)

			publicResourcesShouldNotExist(ctx, svc)

		})
	})

	When("export annotation is removed from previously exported component", func() {

		It("should remove all created routing resources", func(ctx context.Context) {
			// given
			// required annotation for watched custom resource:
			// routing.opendatahub.io/export-mode-external: "true"
			// routing.opendatahub.io/export-mode-public: "true"
			component, createErr := createComponentRequiringPlatformRouting(ctx, "public-and-external-remove-annotation",
				appNs.Name, annotations.ExternalMode(), annotations.PublicMode())
			Expect(createErr).ToNot(HaveOccurred())

			// required labels for the exported service:
			// 	routing.opendatahub.io/exported: "true"
			// 	platform.opendatahub.io/owner-name: test-component
			// 	platform.opendatahub.io/owner-kind: Component
			addRoutingRequirementsToSvc(ctx, svc, component)

			// then
			externalResourcesShouldExist(ctx, svc)
			publicResourcesShouldExist(ctx, svc)

			// when
			By("removing the export annotations", func() {
				setExportModes(ctx, component, removeExternal, removePublic)
			})

			// then
			externalResourcesShouldNotExist(ctx, svc)
			publicResourcesShouldNotExist(ctx, svc)

			Eventually(hasNoAddressAnnotations(component)).
				WithContext(ctx).
				WithTimeout(test.DefaultTimeout).
				WithPolling(test.DefaultPolling).
				Should(Succeed())
		})
	})

	When("export annotation is changed on existing exported component", func() {

		It("should remove all routing resources from removed", func(ctx context.Context) {
			// given
			// required annotation for watched custom resource:
			// routing.opendatahub.io/export-mode-external: "true"
			// routing.opendatahub.io/export-mode-public: "true"
			component, createErr := createComponentRequiringPlatformRouting(ctx, "public-and-external-change-annotation",
				appNs.Name, annotations.ExternalMode(), annotations.PublicMode())
			Expect(createErr).ToNot(HaveOccurred())

			// required labels for the exported service:
			// 	routing.opendatahub.io/exported: "true"
			// 	platform.opendatahub.io/owner-name: test-component
			// 	platform.opendatahub.io/owner-kind: Component
			addRoutingRequirementsToSvc(ctx, svc, component)

			// then
			externalResourcesShouldExist(ctx, svc)
			publicResourcesShouldExist(ctx, svc)

			By("removing external from the export modes", func() {
				setExportModes(ctx, component, enablePublic, disableExternal)
			})

			// then
			externalResourcesShouldNotExist(ctx, svc)
			publicResourcesShouldExist(ctx, svc)

			Eventually(func(g Gomega, ctx context.Context) error {
				updatedComponent := component.DeepCopy()
				if errGet := envTest.Get(ctx, client.ObjectKeyFromObject(updatedComponent), updatedComponent); errGet != nil {
					return errGet
				}

				g.Expect(updatedComponent.GetAnnotations()).ToNot(
					HaveKey(
						annotations.RoutingAddressesExternal("").Key(),
					), "public services are not expected to be defined in this mode")

				publicAddressAnnotation := annotations.RoutingAddressesPublic(
					fmt.Sprintf("%[1]s-http-%[2]s.%[3]s;%[1]s-http-%[2]s.%[3]s.svc;%[1]s-http-%[2]s.%[3]s.svc.cluster.local;"+
						"%[1]s-grpc-%[2]s.%[3]s;%[1]s-grpc-%[2]s.%[3]s.svc;%[1]s-grpc-%[2]s.%[3]s.svc.cluster.local",
						svc.Name, svc.Namespace, routingConfiguration.GatewayNamespace),
				)

				g.Expect(updatedComponent.GetAnnotations()).To(
					HaveKeyWithValue(publicAddressAnnotation.Key(), publicAddressAnnotation.Value()),
				)

				return nil
			}).
				WithContext(ctx).
				WithTimeout(test.DefaultTimeout).
				WithPolling(test.DefaultPolling).
				Should(Succeed())
		})

		It("should remove all routing resources when the unsupported mode is used", func(ctx context.Context) {
			// given
			// required annotation for watched custom resource:
			// routing.opendatahub.io/export-mode-external: "true"
			// routing.opendatahub.io/export-mode-public: "true"
			component, createErr := createComponentRequiringPlatformRouting(ctx, "public-and-external-changed-to-non-existing",
				appNs.Name, annotations.ExternalMode(), annotations.PublicMode())
			Expect(createErr).ToNot(HaveOccurred())

			// required labels for the exported service:
			// 	routing.opendatahub.io/exported: "true"
			// 	platform.opendatahub.io/owner-name: test-component
			// 	platform.opendatahub.io/owner-kind: Component
			addRoutingRequirementsToSvc(ctx, svc, component)

			// then
			externalResourcesShouldExist(ctx, svc)
			publicResourcesShouldExist(ctx, svc)

			By("setting a non-supported mode", func() {
				setExportModes(ctx, component, enableNonSupportedMode, removePublic, removeExternal)
			})

			// then
			externalResourcesShouldNotExist(ctx, svc)
			publicResourcesShouldNotExist(ctx, svc)

			Eventually(hasNoAddressAnnotations(component)).
				WithContext(ctx).
				WithTimeout(test.DefaultTimeout).
				WithPolling(test.DefaultPolling).
				Should(Succeed())
		})
	})

})

func externalResourcesShouldExist(ctx context.Context, svc *corev1.Service) {
	Eventually(routeExistsFor(svc)).
		WithContext(ctx).
		WithTimeout(test.DefaultTimeout).
		WithPolling(test.DefaultPolling).
		Should(Succeed())

	Eventually(ingressVirtualServiceExistsFor(svc)).
		WithContext(ctx).
		WithTimeout(test.DefaultTimeout).
		WithPolling(test.DefaultPolling).
		Should(Succeed())
}

func publicResourcesShouldExist(ctx context.Context, svc *corev1.Service) {
	Eventually(publicSvcExistsFor(svc)).
		WithContext(ctx).
		WithTimeout(test.DefaultTimeout).
		WithPolling(test.DefaultPolling).
		Should(Succeed())

	Eventually(publicGatewayExistsFor(svc)).
		WithContext(ctx).
		WithTimeout(test.DefaultTimeout).
		WithPolling(test.DefaultPolling).
		Should(Succeed())

	Eventually(publicVirtualSvcExistsFor(svc)).
		WithContext(ctx).
		WithTimeout(test.DefaultTimeout).
		WithPolling(test.DefaultPolling).
		Should(Succeed())

	Eventually(destinationRuleExistsFor(svc)).
		WithContext(ctx).
		WithTimeout(test.DefaultTimeout).
		WithPolling(test.DefaultPolling).
		Should(Succeed())
}

func externalResourcesShouldNotExist(ctx context.Context, svc *corev1.Service) {
	Eventually(routeExistsFor(svc)).
		WithContext(ctx).
		WithTimeout(test.DefaultTimeout).
		WithPolling(test.DefaultPolling).
		Should(WithTransform(k8serr.IsNotFound, BeTrue()))

	Eventually(ingressVirtualServiceExistsFor(svc)).
		WithContext(ctx).
		WithTimeout(test.DefaultTimeout).
		WithPolling(test.DefaultPolling).
		Should(WithTransform(k8serr.IsNotFound, BeTrue()))
}

func publicResourcesShouldNotExist(ctx context.Context, svc *corev1.Service) {
	Eventually(publicSvcExistsFor(svc)).
		WithContext(ctx).
		WithTimeout(test.DefaultTimeout).
		WithPolling(test.DefaultPolling).
		Should(WithTransform(k8serr.IsNotFound, BeTrue()))

	Eventually(publicVirtualSvcExistsFor(svc)).
		WithContext(ctx).
		WithTimeout(test.DefaultTimeout).
		WithPolling(test.DefaultPolling).
		Should(WithTransform(k8serr.IsNotFound, BeTrue()))

	Eventually(publicGatewayExistsFor(svc)).
		WithContext(ctx).
		WithTimeout(test.DefaultTimeout).
		WithPolling(test.DefaultPolling).
		Should(WithTransform(k8serr.IsNotFound, BeTrue()))

	Eventually(destinationRuleExistsFor(svc)).
		WithContext(ctx).
		WithTimeout(test.DefaultTimeout).
		WithPolling(test.DefaultPolling).
		Should(WithTransform(k8serr.IsNotFound, BeTrue()))
}

func hasNoAddressAnnotations(component *unstructured.Unstructured) func(g Gomega, ctx context.Context) error {
	return func(g Gomega, ctx context.Context) error {
		updatedComponent := component.DeepCopy()
		Expect(envTest.Get(ctx, client.ObjectKeyFromObject(updatedComponent), updatedComponent)).To(Succeed())

		g.Expect(updatedComponent.GetAnnotations()).ToNot(
			HaveKey(
				annotations.RoutingAddressesExternal("").Key(),
			), "External services are not expected to be defined in this mode")

		g.Expect(updatedComponent.GetAnnotations()).ToNot(
			HaveKey(
				annotations.RoutingAddressesPublic("").Key(),
			), "Public services are not expected to be defined in this mode")

		return nil
	}
}

func routeExistsFor(exposedSvc *corev1.Service) func(g Gomega, ctx context.Context) error {
	return func(g Gomega, ctx context.Context) error {
		g.Expect(exposedSvc.Spec.Ports).ToNot(BeEmpty())

		for _, exposedPort := range exposedSvc.Spec.Ports {
			svcRoute := &openshiftroutev1.Route{}
			if errGet := envTest.Get(ctx, types.NamespacedName{
				Name:      exposedSvc.Name + "-" + exposedPort.Name + "-" + exposedSvc.Namespace + "-route",
				Namespace: routingConfiguration.GatewayNamespace,
			}, svcRoute); errGet != nil {
				return errGet
			}

			g.Expect(svcRoute).To(BeAttachedToService(routingConfiguration.IngressService))
			g.Expect(svcRoute).To(HaveHost(exposedSvc.Name + "-" + exposedPort.Name + "-" + exposedSvc.Namespace + "." + domain))
		}

		return nil
	}
}

func publicSvcExistsFor(exposedSvc *corev1.Service) func(g Gomega, ctx context.Context) error {
	return func(g Gomega, ctx context.Context) error {
		g.Expect(exposedSvc.Spec.Ports).ToNot(BeEmpty())

		for _, exposedPort := range exposedSvc.Spec.Ports {
			publicSvc := &corev1.Service{}
			if errGet := envTest.Get(ctx, types.NamespacedName{
				Name:      exposedSvc.Name + "-" + exposedPort.Name + "-" + exposedSvc.Namespace,
				Namespace: routingConfiguration.GatewayNamespace,
			}, publicSvc); errGet != nil {
				return errGet
			}

			g.Expect(publicSvc.GetAnnotations()).To(
				HaveKeyWithValue(
					"service.beta.openshift.io/serving-cert-secret-name",
					exposedSvc.Name+"-"+exposedPort.Name+"-"+exposedSvc.Namespace+"-certs",
				),
			)

			g.Expect(publicSvc.Spec.Selector).To(
				HaveKeyWithValue(routingConfiguration.IngressSelectorLabel, routingConfiguration.IngressSelectorValue),
			)
		}

		return nil
	}
}

func publicGatewayExistsFor(exposedSvc *corev1.Service) func(g Gomega, ctx context.Context) error {
	return func(g Gomega, ctx context.Context) error {
		g.Expect(exposedSvc.Spec.Ports).ToNot(BeEmpty())

		for _, exposedPort := range exposedSvc.Spec.Ports {
			publicGateway := &v1beta1.Gateway{}
			if errGet := envTest.Get(ctx, types.NamespacedName{
				Name:      exposedSvc.Name + "-" + exposedPort.Name + "-" + exposedSvc.Namespace,
				Namespace: routingConfiguration.GatewayNamespace,
			}, publicGateway); errGet != nil {
				return errGet
			}

			g.Expect(publicGateway.Spec.GetSelector()).To(HaveKeyWithValue(routingConfiguration.IngressSelectorLabel, routingConfiguration.IngressSelectorValue))
			// limitation: only checks first element of []*Server slice
			g.Expect(publicGateway).To(
				HaveHosts(
					exposedSvc.Name+"-"+exposedPort.Name+"-"+exposedSvc.Namespace+"."+routingConfiguration.GatewayNamespace,
					exposedSvc.Name+"-"+exposedPort.Name+"-"+exposedSvc.Namespace+"."+routingConfiguration.GatewayNamespace+".svc",
					exposedSvc.Name+"-"+exposedPort.Name+"-"+exposedSvc.Namespace+"."+routingConfiguration.GatewayNamespace+".svc.cluster.local",
				),
			)
		}

		return nil
	}
}

func publicVirtualSvcExistsFor(exposedSvc *corev1.Service) func(g Gomega, ctx context.Context) error {
	return func(g Gomega, ctx context.Context) error {
		publicVS := &v1beta1.VirtualService{}

		g.Expect(exposedSvc.Spec.Ports).ToNot(BeEmpty())

		for _, exposedPort := range exposedSvc.Spec.Ports {
			if errGet := envTest.Get(ctx, types.NamespacedName{
				Name:      exposedSvc.Name + "-" + exposedPort.Name + "-" + exposedSvc.Namespace,
				Namespace: routingConfiguration.GatewayNamespace,
			}, publicVS); errGet != nil {
				return errGet
			}

			g.Expect(publicVS).To(
				HaveHosts(
					exposedSvc.Name+"-"+exposedPort.Name+"-"+exposedSvc.Namespace+"."+routingConfiguration.GatewayNamespace,
					exposedSvc.Name+"-"+exposedPort.Name+"-"+exposedSvc.Namespace+"."+routingConfiguration.GatewayNamespace+".svc",
					exposedSvc.Name+"-"+exposedPort.Name+"-"+exposedSvc.Namespace+"."+routingConfiguration.GatewayNamespace+".svc.cluster.local",
				),
			)
			g.Expect(publicVS).To(BeAttachedToGateways("mesh", exposedSvc.Name+"-"+exposedPort.Name+"-"+exposedSvc.Namespace))
			g.Expect(publicVS).To(RouteToHost(exposedSvc.Name+"."+exposedSvc.Namespace+".svc.cluster.local", uint32(exposedPort.TargetPort.IntVal)))
		}

		return nil
	}
}

func destinationRuleExistsFor(exposedSvc *corev1.Service) func(g Gomega, ctx context.Context) error {
	return func(g Gomega, ctx context.Context) error {
		g.Expect(exposedSvc.Spec.Ports).ToNot(BeEmpty())

		for _, exposedPort := range exposedSvc.Spec.Ports {
			destinationRule := &v1beta1.DestinationRule{}
			if errGet := envTest.Get(ctx, types.NamespacedName{
				Name:      exposedSvc.Name + "-" + exposedPort.Name + "-" + exposedSvc.Namespace,
				Namespace: routingConfiguration.GatewayNamespace,
			}, destinationRule); errGet != nil {
				return errGet
			}

			g.Expect(destinationRule).To(
				HaveHost(
					exposedSvc.Name + "-" + exposedPort.Name + "-" + exposedSvc.Namespace + "." + routingConfiguration.GatewayNamespace + ".svc.cluster.local",
				),
			)
			g.Expect(destinationRule.Spec.GetTrafficPolicy().GetTls().GetMode()).To(Equal(istionetworkingv1beta1.ClientTLSSettings_DISABLE))
		}

		return nil
	}
}

func ingressVirtualServiceExistsFor(exposedSvc *corev1.Service) func(g Gomega, ctx context.Context) error {
	return func(g Gomega, ctx context.Context) error {
		g.Expect(exposedSvc.Spec.Ports).ToNot(BeEmpty())

		for _, exposedPort := range exposedSvc.Spec.Ports {
			routerVS := &v1beta1.VirtualService{}
			if errGet := envTest.Get(ctx, types.NamespacedName{
				Name:      exposedSvc.Name + "-" + exposedPort.Name + "-" + exposedSvc.Namespace + "-ingress",
				Namespace: routingConfiguration.GatewayNamespace,
			}, routerVS); errGet != nil {
				return errGet
			}

			g.Expect(routerVS).To(HaveHost(exposedSvc.Name + "-" + exposedPort.Name + "-" + exposedSvc.Namespace + "." + domain))
			g.Expect(routerVS).To(BeAttachedToGateways(routingConfiguration.IngressService))
			g.Expect(routerVS).To(RouteToHost(exposedSvc.Name+"."+exposedSvc.Namespace+".svc.cluster.local", uint32(exposedPort.TargetPort.IntValue())))
		}

		return nil
	}
}

func componentResource(name, namespace string) []byte {
	return []byte(fmt.Sprintf(watchedCR, name, namespace))
}

func simpleSvcDeployment(ctx context.Context, nsName, svcName string) (*appsv1.Deployment, *corev1.Service) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcName,
			Namespace: nsName,
			Labels: map[string]string{
				"app":     svcName,
				"service": svcName,
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": svcName,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       8080,
					TargetPort: intstr.FromInt32(8000),
				},
				{
					Name:       "grpc",
					Port:       9080,
					TargetPort: intstr.FromInt32(9000),
				},
			},
		},
	}

	Expect(envTest.Create(ctx, service)).To(Succeed())

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcName,
			Namespace: nsName,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To[int32](1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":     svcName,
					"version": "v1",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"sidecar.istio.io/inject": "true",
					},
					Labels: map[string]string{
						"app":     svcName,
						"version": "v1",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: svcName,
					Containers: []corev1.Container{
						{
							Name:  "httpbin",
							Image: "kennethreitz/httpbin",
							Command: []string{
								"gunicorn", "--access-logfile", "-", "-b", "[::]:8000", "httpbin:app",
							},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 8000,
								},
							},
						},
					},
				},
			},
		},
	}
	Expect(envTest.Create(ctx, deployment)).To(Succeed())

	return deployment, service
}
