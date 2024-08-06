package authorization_test

import (
	"context"
	"fmt"
	"time"

	authorinov1beta2 "github.com/kuadrant/authorino/api/v1beta2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/opendatahub-io/odh-platform/pkg/metadata"
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
		Eventually(func() error {
			createdAuthConfig := &authorinov1beta2.AuthConfig{}
			err := envTest.Client.Get(ctx, types.NamespacedName{
				Name:      resourceName,
				Namespace: testNamespaceName,
			}, createdAuthConfig)

			if err != nil {
				return err
			}

			Expect(createdAuthConfig).To(HaveHosts("example.com"))
			Expect(createdAuthConfig.Labels).To(HaveKeyWithValue("security.opendatahub.io/authorization-group", "default"))

			Expect(createdAuthConfig).To(HaveAuthenticationMethod("anonymous-access"))
			Expect(createdAuthConfig).NotTo(HaveAuthenticationMethod("kubernetes-user"))
			Expect(createdAuthConfig).NotTo(HaveKubernetesTokenReview())

			return nil
		}, 10*time.Second, 2*time.Second).Should(Succeed())
	})

	It("should create a non-anonymous AuthConfig resource when annotation is specified", func(ctx context.Context) {
		if createdComponent.GetAnnotations() == nil {
			createdComponent.SetAnnotations(map[string]string{})
		}

		_, errCreate := controllerutil.CreateOrUpdate(ctx, envTest.Client, createdComponent, func() error {
			return metadata.ApplyMetaOptions(createdComponent, metadata.WithAnnotations(metadata.Annotations.AuthEnabled, "true"))
		})
		Expect(errCreate).ToNot(HaveOccurred())

		Eventually(func() error {
			createdAuthConfig := &authorinov1beta2.AuthConfig{}
			err := envTest.Client.Get(ctx, types.NamespacedName{
				Name:      resourceName,
				Namespace: testNamespaceName,
			}, createdAuthConfig)

			if err != nil {
				return err
			}

			Expect(createdAuthConfig).To(HaveHosts("example.com"))
			Expect(createdAuthConfig.Labels).To(HaveKeyWithValue("security.opendatahub.io/authorization-group", "default"))

			Expect(createdAuthConfig).To(HaveAuthenticationMethod("kubernetes-user"))
			Expect(createdAuthConfig).NotTo(HaveAuthenticationMethod("anonymous-access"))
			Expect(createdAuthConfig).To(HaveKubernetesTokenReview())

			return nil
		}, 10*time.Second, 2*time.Second).Should(Succeed())
	})

	It("should create an AuthorizationPolicy when a Component is created", func(ctx context.Context) {
		Eventually(func() error {
			createdAuthPolicy := &istiosecurityv1beta1.AuthorizationPolicy{}
			err := envTest.Client.Get(ctx, types.NamespacedName{
				Name:      resourceName,
				Namespace: testNamespaceName,
			}, createdAuthPolicy)

			if err != nil {
				return err
			}

			Expect(createdAuthPolicy.Spec.GetAction()).To(Equal(v1beta1.AuthorizationPolicy_CUSTOM))

			return nil
		}, 10*time.Second, 2*time.Second).Should(Succeed())
	})

	It("should create a PeerAuthentication when a Component is created", func(ctx context.Context) {
		Eventually(func() error {
			createdPeerAuth := &istiosecurityv1beta1.PeerAuthentication{}
			err := envTest.Client.Get(ctx, types.NamespacedName{
				Name:      resourceName,
				Namespace: testNamespaceName,
			}, createdPeerAuth)

			if err != nil {
				return err
			}

			Expect(createdPeerAuth.Spec.GetMtls().GetMode()).To(Equal(v1beta1.PeerAuthentication_MutualTLS_PERMISSIVE))

			return nil
		}, 10*time.Second, 2*time.Second).Should(Succeed())
	})

	// TODO: fill out stubs once owner-name labels are propagated to auth resources
	PIt("should have ownerReference on all created auth resources", func(ctx context.Context) {
		// get all three resources by owner name label

		// use matchers.owner.go to ensure that correct ownerReference is set on all of them
	})
})

func componentResource(name, namespace string) []byte {
	return []byte(fmt.Sprintf(watchedCR, name, namespace))
}
