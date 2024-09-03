package authzctrl_test

import (
	"context"
	"fmt"

	authorinov1beta2 "github.com/kuadrant/authorino/api/v1beta2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/opendatahub-io/odh-platform/pkg/metadata"
	"github.com/opendatahub-io/odh-platform/pkg/metadata/annotations"
	"github.com/opendatahub-io/odh-platform/test"
	. "github.com/opendatahub-io/odh-platform/test/matchers"
	"istio.io/api/security/v1beta1"
	istiosecurityv1beta1 "istio.io/client-go/pkg/apis/security/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
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
  host: example.com
`

var _ = Describe("Checking Authorization Resource Creation", test.EnvTest(), func() {
	var (
		resourceName      string
		testNamespaceName string
		testNamespace     *corev1.Namespace
		createdComponent  *unstructured.Unstructured
	)

	BeforeEach(func(ctx context.Context) {
		resourceName = "test-component"
		base := "test-namespace"
		testNamespaceName = fmt.Sprintf("%s%s", base, utilrand.String(7))

		testNamespace = &corev1.Namespace{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Namespace",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespaceName,
			},
		}
		_, err := controllerutil.CreateOrUpdate(ctx, envTest.Client, testNamespace, func() error {
			return nil
		})
		Expect(err).ToNot(HaveOccurred())

		var errCreate error
		createdComponent, errCreate = test.CreateResource(ctx, envTest.Client, componentResource(resourceName, testNamespaceName))
		Expect(errCreate).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		envTest.DeleteAll(createdComponent, testNamespace)
	})

	It("should create an anonymous AuthConfig resource by default when a target CR is created", func(ctx context.Context) {
		Eventually(func(g Gomega, ctx context.Context) error {
			createdAuthConfig := &authorinov1beta2.AuthConfig{}
			err := envTest.Client.Get(ctx, types.NamespacedName{
				Name:      resourceName,
				Namespace: testNamespaceName,
			}, createdAuthConfig)

			if err != nil {
				return err
			}

			g.Expect(createdAuthConfig).To(HaveHosts("example.com"))
			g.Expect(createdAuthConfig.Labels).To(HaveKeyWithValue("security.opendatahub.io/authorization-group", "default"))

			g.Expect(createdAuthConfig).To(HaveAuthenticationMethod("anonymous-access"))
			g.Expect(createdAuthConfig).NotTo(HaveAuthenticationMethod("kubernetes-user"))
			g.Expect(createdAuthConfig).NotTo(HaveKubernetesTokenReview())

			return nil
		}).
			WithContext(ctx).
			WithTimeout(test.DefaultTimeout).
			WithPolling(test.DefaultPolling).
			Should(Succeed())
	})

	It("should create a non-anonymous AuthConfig resource when annotation is specified", func(ctx context.Context) {
		if createdComponent.GetAnnotations() == nil {
			createdComponent.SetAnnotations(map[string]string{})
		}

		_, errCreate := controllerutil.CreateOrUpdate(ctx, envTest.Client, createdComponent, func() error {
			metadata.ApplyMetaOptions(createdComponent, annotations.AuthEnabled("true"))

			return nil
		})
		Expect(errCreate).ToNot(HaveOccurred())

		Eventually(func(g Gomega, ctx context.Context) error {
			createdAuthConfig := &authorinov1beta2.AuthConfig{}
			err := envTest.Client.Get(ctx, types.NamespacedName{
				Name:      resourceName,
				Namespace: testNamespaceName,
			}, createdAuthConfig)

			if err != nil {
				return err
			}

			g.Expect(createdAuthConfig).To(HaveHosts("example.com"))
			g.Expect(createdAuthConfig.Labels).To(HaveKeyWithValue("security.opendatahub.io/authorization-group", "default"))

			g.Expect(createdAuthConfig).To(HaveAuthenticationMethod("kubernetes-user"))
			g.Expect(createdAuthConfig).NotTo(HaveAuthenticationMethod("anonymous-access"))
			g.Expect(createdAuthConfig).To(HaveKubernetesTokenReview())

			return nil
		}).
			WithContext(ctx).
			WithTimeout(test.DefaultTimeout).
			WithPolling(test.DefaultPolling).
			Should(Succeed())
	})

	It("should create an AuthorizationPolicy when a Component is created", func(ctx context.Context) {
		Eventually(func(g Gomega, ctx context.Context) error {
			createdAuthPolicy := &istiosecurityv1beta1.AuthorizationPolicy{}
			err := envTest.Client.Get(ctx, types.NamespacedName{
				Name:      resourceName,
				Namespace: testNamespaceName,
			}, createdAuthPolicy)

			if err != nil {
				return err
			}

			g.Expect(createdAuthPolicy.Spec.GetAction()).To(Equal(v1beta1.AuthorizationPolicy_CUSTOM))
			// WorkloadSelector expresssion defined in suite_test
			g.Expect(createdAuthPolicy.Spec.GetSelector().GetMatchLabels()).To(HaveKeyWithValue("component", resourceName))

			return nil
		}).
			WithContext(ctx).
			WithTimeout(test.DefaultTimeout).
			WithPolling(test.DefaultPolling).
			Should(Succeed())
	})

	// Using k8s envtest we are not able to test actual garbage collection of resources. [1]
	// Therefore, we ensure we have correct ownerRefs set.
	//
	// [1] https://book.kubebuilder.io/reference/envtest#testing-considerations
	It("should have ownerReference on all created auth resources", func(ctx context.Context) {
		Eventually(func(g Gomega, ctx context.Context) error {
			createdResource := &istiosecurityv1beta1.AuthorizationPolicy{}
			err := envTest.Client.Get(ctx, types.NamespacedName{
				Name:      resourceName,
				Namespace: testNamespaceName,
			}, createdResource)

			if err != nil {
				return err
			}

			ctrl := true
			expectedOwnerRef := metav1.OwnerReference{
				APIVersion: createdComponent.GetAPIVersion(),
				Kind:       createdComponent.GetKind(),
				Name:       createdComponent.GetName(),
				UID:        createdComponent.GetUID(),
				Controller: &ctrl,
			}

			g.Expect(createdResource.OwnerReferences).To(ContainElement(expectedOwnerRef))

			return nil
		}).
			WithContext(ctx).
			WithTimeout(test.DefaultTimeout).
			WithPolling(test.DefaultPolling).
			Should(Succeed())
	})
})

func componentResource(name, namespace string) []byte {
	return []byte(fmt.Sprintf(watchedCR, name, namespace))
}
