package routing_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/opendatahub-io/odh-platform/pkg/metadata"
	"github.com/opendatahub-io/odh-platform/test"
	. "github.com/opendatahub-io/odh-platform/test/matchers"
	openshiftroutev1 "github.com/openshift/api/route/v1"
	istionetworkingv1beta1 "istio.io/api/networking/v1beta1"
	"istio.io/client-go/pkg/apis/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

var _ = Describe("Platform routing setup for the component", test.EnvTest(), func() {

	var (
		routerNs   *corev1.Namespace
		appNs      *corev1.Namespace
		deployment *appsv1.Deployment
		svc        *corev1.Service
		domain     string

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
		testNamespaceName := fmt.Sprintf("%s%s", base, utilrand.String(7))

		appNs = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespaceName,
			},
		}
		Expect(envTest.Client.Create(ctx, appNs)).To(Succeed())

		deployment, svc = simpleSvcDeployment(ctx, appNs.Name, "mesh-service-name")

		toRemove = []client.Object{routerNs, deployment, svc}

		if !envTest.UsingExistingCluster() {
			config, errIngress := test.DefaultIngressControllerConfig(ctx, envTest.Client)
			Expect(errIngress).ToNot(HaveOccurred())
			toRemove = append(toRemove, config)
		}

		domain = getClusterDomain(ctx)
	})

	AfterEach(func(_ context.Context) {
		envTest.DeleteAll(toRemove...)
	})

	When("watched component requests to expose service externally to the cluster", func() {

		It("should have external routing resources created", func(ctx context.Context) {
			// given
			component, errCreate := test.CreateResource(ctx, envTest.Client,
				componentResource("exported-test-component", appNs.Name))
			Expect(errCreate).ToNot(HaveOccurred())
			toRemove = append(toRemove, component)

			// when routing through platform mesh
			By("adding component service to platform routing", func() {
				// before updating component, ensure finalizers have been set
				component = ensureFinalizersSet(ctx, component)

				// required annotation for watched custom resource:
				// 	routing.opendatahub.io/export-mode: "external"
				exportCustomResource(ctx, component, "external")

				// required labels for the exported service:
				// 	routing.opendatahub.io/exported: "true"
				// 	platform.opendatahub.io/owner-name: test-component
				// 	platform.opendatahub.io/owner-kind: Component
				addRoutingRequirementsToSvc(ctx, svc, component)
			})

			// then
			externalResourcesShouldExist(ctx, domain, svc)
		})

		It("should have new hosts propagated back to watched resource", func(ctx context.Context) {
			// given
			component, errCreate := test.CreateResource(ctx, envTest.Client,
				componentResource("exported-test-component", appNs.Name))
			Expect(errCreate).ToNot(HaveOccurred())
			toRemove = append(toRemove, component)

			// when routing through platform mesh
			By("adding component service to platform routing", func() {
				// before updating component, ensure finalizers have been set
				component = ensureFinalizersSet(ctx, component)

				// required annotation for watched custom resource:
				// 	routing.opendatahub.io/export-mode: "external"
				exportCustomResource(ctx, component, "external")

				// required labels for the exported service:
				// 	routing.opendatahub.io/exported: "true"
				// 	platform.opendatahub.io/owner-name: test-component
				// 	platform.opendatahub.io/owner-kind: Component
				addRoutingRequirementsToSvc(ctx, svc, component)
			})

			// then
			Eventually(func(g Gomega, ctx context.Context) error {
				updatedComponent := component.DeepCopy()
				if errGet := envTest.Get(ctx, client.ObjectKeyFromObject(updatedComponent), updatedComponent); errGet != nil {
					return errGet
				}

				g.Expect(updatedComponent.GetAnnotations()).ToNot(HaveKey(
					metadata.Annotations.RoutingAddressesPublic,
				), "public services are not expected to be defined in this mode")

				g.Expect(updatedComponent.GetAnnotations()).To(HaveKeyWithValue(
					metadata.Annotations.RoutingAddressesExternal, fmt.Sprintf("%s-%s.%s", svc.Name, svc.Namespace, domain),
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
			component, errCreate := test.CreateResource(ctx, envTest.Client,
				componentResource("public-test-component", appNs.Name))
			Expect(errCreate).ToNot(HaveOccurred())
			toRemove = append(toRemove, component)

			// when routing through platform mesh
			By("adding component service to platform routing", func() {
				// before updating component, ensure finalizers have been set
				component = ensureFinalizersSet(ctx, component)

				// required annotation for watched custom resource:
				// 	routing.opendatahub.io/export-mode: "public"
				exportCustomResource(ctx, component, "public")

				// required labels for the exported service:
				// 	routing.opendatahub.io/exported: "true"
				// 	platform.opendatahub.io/owner-name: test-component
				// 	platform.opendatahub.io/owner-kind: Component
				addRoutingRequirementsToSvc(ctx, svc, component)
			})

			// then
			publicResourcesShouldExist(ctx, svc)

		})

		It("should have new hosts propagated back to watched resource by the controller", func(ctx context.Context) {
			// given
			component, errCreate := test.CreateResource(ctx, envTest.Client,
				componentResource("public-test-component", appNs.Name))
			Expect(errCreate).ToNot(HaveOccurred())
			toRemove = append(toRemove, component)

			// when routing through platform mesh
			By("adding component service to platform routing", func() {
				// before updating component, ensure finalizers have been set
				component = ensureFinalizersSet(ctx, component)

				// required annotation for watched custom resource:
				// 	routing.opendatahub.io/export-mode: "public"
				exportCustomResource(ctx, component, "public")

				// required labels for the exported service:
				// 	routing.opendatahub.io/exported: "true"
				// 	platform.opendatahub.io/owner-name: test-component
				// 	platform.opendatahub.io/owner-kind: Component
				addRoutingRequirementsToSvc(ctx, svc, component)
			})

			// then
			Eventually(func(g Gomega, ctx context.Context) error {
				updatedComponent := component.DeepCopy()
				if errGet := envTest.Get(ctx, client.ObjectKeyFromObject(updatedComponent), updatedComponent); errGet != nil {
					return errGet
				}

				g.Expect(updatedComponent.GetAnnotations()).ToNot(
					HaveKey(
						metadata.Annotations.RoutingAddressesExternal,
					), "public services are not expected to be defined in this mode")

				g.Expect(updatedComponent.GetAnnotations()).To(
					HaveKeyWithValue(
						metadata.Annotations.RoutingAddressesPublic,
						fmt.Sprintf("%[1]s-%[2]s.%[3]s;%[1]s-%[2]s.%[3]s.svc;%[1]s-%[2]s.%[3]s.svc.cluster.local", svc.Name, svc.Namespace, routingConfiguration.GatewayNamespace),
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

	When("component requests to expose service both locally and externally to the cluster", func() {

		It("should have both external and cluster-local resources created", func(ctx context.Context) {
			// given
			component, errCreate := test.CreateResource(ctx, envTest.Client,
				componentResource("public-and-external-test-component", appNs.Name))
			Expect(errCreate).ToNot(HaveOccurred())
			toRemove = append(toRemove, component)

			// when routing through platform mesh
			By("adding component service to platform routing", func() {
				// before updating component, ensure finalizers have been set
				component = ensureFinalizersSet(ctx, component)

				// required annotation for watched custom resource:
				// 	routing.opendatahub.io/export-mode: "public;external"
				exportCustomResource(ctx, component, "public;external")

				// required labels for the exported service:
				// 	routing.opendatahub.io/exported: "true"
				// 	platform.opendatahub.io/owner-name: test-component
				// 	platform.opendatahub.io/owner-kind: Component
				addRoutingRequirementsToSvc(ctx, svc, component)
			})

			// then
			externalResourcesShouldExist(ctx, domain, svc)
			publicResourcesShouldExist(ctx, svc)

			Eventually(func(g Gomega, ctx context.Context) error {
				updatedComponent := component.DeepCopy()
				if errGet := envTest.Get(ctx, client.ObjectKeyFromObject(updatedComponent), updatedComponent); errGet != nil {
					return errGet
				}

				g.Expect(updatedComponent.GetAnnotations()).To(
					And(
						HaveKeyWithValue(
							metadata.Annotations.RoutingAddressesExternal,
							fmt.Sprintf("%s-%s.%s", svc.Name, svc.Namespace, domain),
						),
						HaveKeyWithValue(
							metadata.Annotations.RoutingAddressesPublic,
							fmt.Sprintf("%[1]s-%[2]s.%[3]s;%[1]s-%[2]s.%[3]s.svc;%[1]s-%[2]s.%[3]s.svc.cluster.local", svc.Name, svc.Namespace, routingConfiguration.GatewayNamespace),
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
			component, errCreate := test.CreateResource(ctx, envTest.Client,
				componentResource("public-and-external-test-deletion", appNs.Name))
			Expect(errCreate).ToNot(HaveOccurred())
			toRemove = append(toRemove, component)

			// when
			By("adding routing requirements on the resource and related svc", func() {
				// before updating component, ensure finalizers have been set
				component = ensureFinalizersSet(ctx, component)

				// required annotation for watched custom resource:
				// 	routing.opendatahub.io/export-mode: "public;external"
				exportCustomResource(ctx, component, "public;external")

				// required labels for the exported service:
				// 	routing.opendatahub.io/exported: "true"
				// 	platform.opendatahub.io/owner-name: test-component
				// 	platform.opendatahub.io/owner-kind: Component
				addRoutingRequirementsToSvc(ctx, svc, component)
			})

			// then
			externalResourcesShouldExist(ctx, domain, svc)
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
			Eventually(routeExistsFor(svc, domain)).
				WithContext(ctx).
				WithTimeout(test.DefaultTimeout).
				WithPolling(test.DefaultPolling).
				ShouldNot(Succeed())

			Eventually(ingressVirtualServiceExistsFor(svc, domain)).
				WithContext(ctx).
				WithTimeout(test.DefaultTimeout).
				WithPolling(test.DefaultPolling).
				ShouldNot(Succeed())

			Eventually(publicSvcExistsFor(svc)).
				WithContext(ctx).
				WithTimeout(test.DefaultTimeout).
				WithPolling(test.DefaultPolling).
				ShouldNot(Succeed())

			Eventually(publicVirtualSvcExistsFor(svc)).
				WithContext(ctx).
				WithTimeout(test.DefaultTimeout).
				WithPolling(test.DefaultPolling).
				ShouldNot(Succeed())

			Eventually(publicGatewayExistsFor(svc)).
				WithContext(ctx).
				WithTimeout(test.DefaultTimeout).
				WithPolling(test.DefaultPolling).
				ShouldNot(Succeed())
		})
	})

})

func externalResourcesShouldExist(ctx context.Context, domain string, svc *corev1.Service) {
	Eventually(routeExistsFor(svc, domain)).
		WithContext(ctx).
		WithTimeout(test.DefaultTimeout).
		WithPolling(test.DefaultPolling).
		Should(Succeed())

	Eventually(ingressVirtualServiceExistsFor(svc, domain)).
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

func routeExistsFor(exportedSvc *corev1.Service, domain string) func(g Gomega, ctx context.Context) error {
	return func(g Gomega, ctx context.Context) error {
		svcRoute := &openshiftroutev1.Route{}
		if errGet := envTest.Get(ctx, types.NamespacedName{
			Name:      exportedSvc.Name + "-" + exportedSvc.Namespace + "-route",
			Namespace: routingConfiguration.GatewayNamespace,
		}, svcRoute); errGet != nil {
			return errGet
		}

		g.Expect(svcRoute).To(BeAttachedToService(routingConfiguration.IngressService))
		g.Expect(svcRoute).To(HaveHost(exportedSvc.Name + "-" + exportedSvc.Namespace + "." + domain))

		return nil
	}
}

func publicSvcExistsFor(exposedSvc *corev1.Service) func(g Gomega, ctx context.Context) error {
	return func(g Gomega, ctx context.Context) error {
		publicSvc := &corev1.Service{}
		if errGet := envTest.Get(ctx, types.NamespacedName{
			Name:      exposedSvc.Name + "-" + exposedSvc.Namespace,
			Namespace: routingConfiguration.GatewayNamespace,
		}, publicSvc); errGet != nil {
			return errGet
		}

		g.Expect(publicSvc.GetAnnotations()).To(
			HaveKeyWithValue(
				"service.beta.openshift.io/serving-cert-secret-name",
				exposedSvc.Name+"-"+exposedSvc.Namespace+"-certs",
			),
		)

		g.Expect(publicSvc.Spec.Selector).To(
			HaveKeyWithValue(routingConfiguration.IngressSelectorLabel, routingConfiguration.IngressSelectorValue),
		)

		return nil
	}
}

func publicGatewayExistsFor(exposedSvc *corev1.Service) func(g Gomega, ctx context.Context) error {
	return func(g Gomega, ctx context.Context) error {
		publicGateway := &v1beta1.Gateway{}
		if errGet := envTest.Get(ctx, types.NamespacedName{
			Name:      exposedSvc.Name + "-" + exposedSvc.Namespace,
			Namespace: routingConfiguration.GatewayNamespace,
		}, publicGateway); errGet != nil {
			return errGet
		}

		g.Expect(publicGateway.Spec.GetSelector()).To(HaveKeyWithValue(routingConfiguration.IngressSelectorLabel, routingConfiguration.IngressSelectorValue))
		// limitation: only checks first element of []*Server slice
		g.Expect(publicGateway).To(
			HaveHosts(
				exposedSvc.Name+"-"+exposedSvc.Namespace+"."+routingConfiguration.GatewayNamespace,
				exposedSvc.Name+"-"+exposedSvc.Namespace+"."+routingConfiguration.GatewayNamespace+".svc",
				exposedSvc.Name+"-"+exposedSvc.Namespace+"."+routingConfiguration.GatewayNamespace+".svc.cluster.local",
			),
		)

		return nil
	}
}

func publicVirtualSvcExistsFor(exposedSvc *corev1.Service) func(g Gomega, ctx context.Context) error {
	return func(g Gomega, ctx context.Context) error {
		publicVS := &v1beta1.VirtualService{}
		if errGet := envTest.Get(ctx, types.NamespacedName{
			Name:      exposedSvc.Name + "-" + exposedSvc.Namespace,
			Namespace: routingConfiguration.GatewayNamespace,
		}, publicVS); errGet != nil {
			return errGet
		}

		g.Expect(publicVS).To(
			HaveHosts(
				exposedSvc.Name+"-"+exposedSvc.Namespace+"."+routingConfiguration.GatewayNamespace,
				exposedSvc.Name+"-"+exposedSvc.Namespace+"."+routingConfiguration.GatewayNamespace+".svc",
				exposedSvc.Name+"-"+exposedSvc.Namespace+"."+routingConfiguration.GatewayNamespace+".svc.cluster.local",
			),
		)
		g.Expect(publicVS).To(BeAttachedToGateways("mesh", exposedSvc.Name+"-"+exposedSvc.Namespace))
		g.Expect(publicVS).To(RouteToHost(exposedSvc.Name+"."+exposedSvc.Namespace+".svc.cluster.local", 8000))

		return nil
	}
}

func destinationRuleExistsFor(exposedSvc *corev1.Service) func(g Gomega, ctx context.Context) error {
	return func(g Gomega, ctx context.Context) error {
		destinationRule := &v1beta1.DestinationRule{}
		if errGet := envTest.Get(ctx, types.NamespacedName{
			Name:      exposedSvc.Name + "-" + exposedSvc.Namespace,
			Namespace: routingConfiguration.GatewayNamespace,
		}, destinationRule); errGet != nil {
			return errGet
		}

		g.Expect(destinationRule).To(
			HaveHost(
				exposedSvc.Name + "-" + exposedSvc.Namespace + "." + routingConfiguration.GatewayNamespace + ".svc.cluster.local",
			),
		)
		g.Expect(destinationRule.Spec.GetTrafficPolicy().GetTls().GetMode()).To(Equal(istionetworkingv1beta1.ClientTLSSettings_DISABLE))

		return nil
	}
}

func ingressVirtualServiceExistsFor(exportedSvc *corev1.Service, domain string) func(g Gomega, ctx context.Context) error {
	return func(g Gomega, ctx context.Context) error {
		routerVS := &v1beta1.VirtualService{}
		if errGet := envTest.Get(ctx, types.NamespacedName{
			Name:      exportedSvc.Name + "-" + exportedSvc.Namespace + "-ingress",
			Namespace: routingConfiguration.GatewayNamespace,
		}, routerVS); errGet != nil {
			return errGet
		}

		g.Expect(routerVS).To(HaveHost(exportedSvc.Name + "-" + exportedSvc.Namespace + "." + domain))
		g.Expect(routerVS).To(BeAttachedToGateways(routingConfiguration.IngressService))
		g.Expect(routerVS).To(RouteToHost(exportedSvc.Name+"."+exportedSvc.Namespace+".svc.cluster.local", 8000))

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
