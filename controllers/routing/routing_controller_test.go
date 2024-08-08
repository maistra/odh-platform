package routing_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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

const domain = "opendatahub.io"

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

		appNs = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "app-ns",
			},
		}
		Expect(envTest.Client.Create(ctx, appNs)).To(Succeed())

		config, errIngress := test.DefaultIngressControllerConfig(ctx, envTest.Client)
		Expect(errIngress).ToNot(HaveOccurred())

		deployment, svc = simpleSvcDeployment(ctx, appNs.Name, "mesh-service-name")

		toRemove = []client.Object{routerNs, appNs, config, svc, deployment}

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

			// when
			By("adding routing requirements on the resource and related svc", func() {
				// routing.opendatahub.io/exported: "true"
				exportAnnotation := metadata.WithLabels(metadata.Labels.RoutingExported, "true")
				// platform.opendatahub.io/owner-name: test-component
				// platform.opendatahub.io/owner-kind: Component
				ownerLabels := metadata.WithOwnerLabels(component)

				// Service created by the component need to have these metadata added, i.e. by its controller
				_, errExportSvc := controllerutil.CreateOrUpdate(ctx, envTest.Client, svc, func() error {
					return metadata.ApplyMetaOptions(svc, exportAnnotation, ownerLabels)
				})
				Expect(errExportSvc).ToNot(HaveOccurred())

				// routing.opendatahub.io/export-mode: "external"
				exposeExternally := metadata.WithAnnotations(metadata.Annotations.RoutingExportMode, "external")
				_, errExportCR := controllerutil.CreateOrUpdate(
					ctx, envTest.Client,
					component,
					func() error {
						return metadata.ApplyMetaOptions(component, exposeExternally)
					})
				Expect(errExportCR).ToNot(HaveOccurred())
			})

			// then
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

		})

		It("should have new hosts propagated back to watched resource", func(ctx context.Context) {
			// given
			component, errCreate := test.CreateResource(ctx, envTest.Client,
				componentResource("exported-test-component", appNs.Name))
			Expect(errCreate).ToNot(HaveOccurred())
			toRemove = append(toRemove, component)

			// when
			By("adding routing requirements on the watched resource and related svc", func() {
				// routing.opendatahub.io/exported: "true"
				exportAnnotation := metadata.WithLabels(metadata.Labels.RoutingExported, "true")
				// platform.opendatahub.io/owner-name: test-component
				// platform.opendatahub.io/owner-kind: Component
				ownerLabels := metadata.WithOwnerLabels(component)

				// Service created by the component need to have these metadata added, i.e. by its controller
				_, errExportSvc := controllerutil.CreateOrUpdate(ctx, envTest.Client, svc, func() error {
					return metadata.ApplyMetaOptions(svc, exportAnnotation, ownerLabels)
				})
				Expect(errExportSvc).ToNot(HaveOccurred())

				// routing.opendatahub.io/export-mode: "external"
				exposeExternally := metadata.WithAnnotations(metadata.Annotations.RoutingExportMode, "external")
				_, errExportCR := controllerutil.CreateOrUpdate(
					ctx, envTest.Client,
					component,
					func() error {
						return metadata.ApplyMetaOptions(component, exposeExternally)
					})
				Expect(errExportCR).ToNot(HaveOccurred())
			})

			// then
			Eventually(func(g Gomega, ctx context.Context) error {
				updatedComponent := component.DeepCopy()
				if errGet := envTest.Get(ctx, client.ObjectKeyFromObject(updatedComponent), updatedComponent); errGet != nil {
					return errGet
				}

				g.Expect(updatedComponent).ToNot(HaveAnnotations(
					metadata.Annotations.RoutingAddressesPublic, gstruct.Ignore(),
				), "public services are not expected to be defined in this mode")

				g.Expect(updatedComponent).To(HaveAnnotations(
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

			// when
			By("adding routing requirements on the resource and related svc", func() {
				// routing.opendatahub.io/exported: "true"
				exportAnnotation := metadata.WithLabels(metadata.Labels.RoutingExported, "true")
				// platform.opendatahub.io/owner-name: test-component
				// platform.opendatahub.io/owner-kind: Component
				ownerLabels := metadata.WithOwnerLabels(component)

				// Service created by the component need to have these metadata added, i.e. by its controller
				_, errExportSvc := controllerutil.CreateOrUpdate(ctx, envTest.Client, svc, func() error {
					return metadata.ApplyMetaOptions(svc, exportAnnotation, ownerLabels)
				})
				Expect(errExportSvc).ToNot(HaveOccurred())

				// routing.opendatahub.io/export-mode: "public"
				exposeExternally := metadata.WithAnnotations(metadata.Annotations.RoutingExportMode, "public")
				_, errExportCR := controllerutil.CreateOrUpdate(
					ctx, envTest.Client,
					component,
					func() error {
						return metadata.ApplyMetaOptions(component, exposeExternally)
					})
				Expect(errExportCR).ToNot(HaveOccurred())
			})

			// then
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

		})

		It("should have new hosts propagated back to watched resource by the controller", func(ctx context.Context) {
			// given
			component, errCreate := test.CreateResource(ctx, envTest.Client,
				componentResource("public-test-component", appNs.Name))
			Expect(errCreate).ToNot(HaveOccurred())
			toRemove = append(toRemove, component)

			// when
			By("adding routing requirements on the resource and related svc", func() {
				// routing.opendatahub.io/exported: "true"
				exportSvc := metadata.WithLabels(metadata.Labels.RoutingExported, "true")
				// platform.opendatahub.io/owner-name: test-component
				// platform.opendatahub.io/owner-kind: Component
				ownerLabels := metadata.WithOwnerLabels(component)

				// Service created by the component need to have these metadata added, i.e. by its controller
				_, errExportSvc := controllerutil.CreateOrUpdate(ctx, envTest.Client, svc, func() error {
					return metadata.ApplyMetaOptions(svc, exportSvc, ownerLabels)
				})
				Expect(errExportSvc).ToNot(HaveOccurred())

				// routing.opendatahub.io/export-mode: "public"
				exposeExternally := metadata.WithAnnotations(metadata.Annotations.RoutingExportMode, "public")
				_, errExportCR := controllerutil.CreateOrUpdate(
					ctx, envTest.Client,
					component,
					func() error {
						return metadata.ApplyMetaOptions(component, exposeExternally)
					})
				Expect(errExportCR).ToNot(HaveOccurred())
			})

			// then
			Eventually(func(g Gomega, ctx context.Context) error {
				updatedComponent := component.DeepCopy()
				if errGet := envTest.Get(ctx, client.ObjectKeyFromObject(updatedComponent), updatedComponent); errGet != nil {
					return errGet
				}

				g.Expect(updatedComponent).ToNot(
					HaveAnnotations(
						metadata.Annotations.RoutingAddressesExternal, gstruct.Ignore(),
					), "public services are not expected to be defined in this mode")

				g.Expect(updatedComponent).To(
					HaveAnnotations(
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

			// when
			By("adding routing requirements on the resource and related svc", func() {
				// routing.opendatahub.io/exported: "true"
				exportAnnotation := metadata.WithLabels(metadata.Labels.RoutingExported, "true")
				// platform.opendatahub.io/owner-name: test-component
				// platform.opendatahub.io/owner-kind: Component
				ownerLabels := metadata.WithOwnerLabels(component)

				// Service created by the component need to have these metadata added, i.e. by its controller
				_, errExportSvc := controllerutil.CreateOrUpdate(ctx, envTest.Client, svc, func() error {
					return metadata.ApplyMetaOptions(svc, exportAnnotation, ownerLabels)
				})
				Expect(errExportSvc).ToNot(HaveOccurred())

				// routing.opendatahub.io/export-mode: "public;external"
				exposeExternally := metadata.WithAnnotations(metadata.Annotations.RoutingExportMode, "public;external")
				_, errExportCR := controllerutil.CreateOrUpdate(
					ctx, envTest.Client,
					component,
					func() error {
						return metadata.ApplyMetaOptions(component, exposeExternally)
					})
				Expect(errExportCR).ToNot(HaveOccurred())
			})

			// then
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

			Eventually(publicSvcExistsFor(svc)).
				WithContext(ctx).
				WithTimeout(test.DefaultTimeout).
				WithPolling(test.DefaultPolling).
				Should(Succeed())

			Eventually(publicVirtualSvcExistsFor(svc)).
				WithContext(ctx).
				WithTimeout(test.DefaultTimeout).
				WithPolling(test.DefaultPolling).
				Should(Succeed())

			Eventually(publicGatewayExistsFor(svc)).
				WithContext(ctx).
				WithTimeout(test.DefaultTimeout).
				WithPolling(test.DefaultPolling).
				Should(Succeed())

			Eventually(destinationRuleExistsFor(svc)).
				WithContext(ctx).
				WithTimeout(test.DefaultTimeout).
				WithPolling(test.DefaultPolling).
				Should(Succeed())

			Eventually(func(g Gomega, ctx context.Context) error {
				updatedComponent := component.DeepCopy()
				if errGet := envTest.Get(ctx, client.ObjectKeyFromObject(updatedComponent), updatedComponent); errGet != nil {
					return errGet
				}

				g.Expect(updatedComponent).To(
					HaveAnnotations(
						metadata.Annotations.RoutingAddressesExternal,
						fmt.Sprintf("%s-%s.%s", svc.Name, svc.Namespace, domain),
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

	PWhen("component is deleted all routing resources should be removed", func() {

	})

})

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

func routeExistsFor(exportedSvc *corev1.Service) func(g Gomega, ctx context.Context) error {
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

		g.Expect(publicSvc).To(
			HaveAnnotations(
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

func ingressVirtualServiceExistsFor(exportedSvc *corev1.Service) func(g Gomega, ctx context.Context) error {
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
