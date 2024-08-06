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
	"istio.io/api/security/v1beta1"
	istiosecurityv1beta1 "istio.io/client-go/pkg/apis/security/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ = Describe("Checking Authorization Resource Creation", test.EnvTest(), func() {
	var (
		resourceName      string
		testNamespaceName string
		testNamespace     *corev1.Namespace
		createdCfgMap     *corev1.ConfigMap
	)

	BeforeEach(func(ctx context.Context) {
		resourceName = "test-configmap"
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

		testConfigMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: testNamespaceName,
			},
			Data: map[string]string{
				"host": "example.com",
			},
		}
		_, err = controllerutil.CreateOrUpdate(ctx, envTest.Client, testConfigMap, func() error {
			return nil
		})
		Expect(err).ToNot(HaveOccurred())

		createdCfgMap = &corev1.ConfigMap{}
		err = envTest.Client.Get(ctx, types.NamespacedName{
			Name:      resourceName,
			Namespace: testNamespaceName,
		}, createdCfgMap)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		envTest.DeleteAll(createdCfgMap, testNamespace)
	})

	// TODO: rename other references to cfgmap to target CR (similar)
	// TODO: test reconcile/update on anonymous vs rules defined (verify both cases)
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

			Expect(createdAuthConfig.ObjectMeta.OwnerReferences).To(HaveLen(1))
			ownerRef := createdAuthConfig.ObjectMeta.OwnerReferences[0]
			checkOwnerRef(ownerRef, *createdCfgMap)

			Expect(createdAuthConfig.Spec.Hosts).To(ContainElement("example.com"))
			Expect(createdAuthConfig.Labels).To(HaveKeyWithValue("security.opendatahub.io/authorization-group", "default"))

			// Check for non-anonymous authentication
			authMethod := createdAuthConfig.Spec.Authentication
			Expect(authMethod).To(HaveKey("anonymous-access"))
			Expect(authMethod).NotTo(HaveKey("kubernetes-user"))

			return nil
		}, 10*time.Second, 2*time.Second).Should(Succeed())
	})

	It("should create a non-anonymous AuthConfig resource when annotation is specified", func(ctx context.Context) {
		if createdCfgMap.Annotations == nil {
			createdCfgMap.Annotations = map[string]string{}
		}

		createdCfgMap.Annotations[metadata.Annotations.AuthEnabled] = "true"
		Expect(envTest.Client.Update(ctx, createdCfgMap)).To(Succeed())

		Eventually(func() error {
			createdAuthConfig := &authorinov1beta2.AuthConfig{}
			err := envTest.Client.Get(ctx, types.NamespacedName{
				Name:      resourceName,
				Namespace: testNamespaceName,
			}, createdAuthConfig)

			if err != nil {
				return err
			}

			Expect(createdAuthConfig.Spec.Hosts).To(ContainElement("example.com"))
			Expect(createdAuthConfig.Labels).To(HaveKeyWithValue("security.opendatahub.io/authorization-group", "default"))

			// Check for non-anonymous authentication
			authMethod := createdAuthConfig.Spec.Authentication
			Expect(authMethod).To(HaveKey("kubernetes-user"))
			Expect(authMethod).NotTo(HaveKey("anonymous-access"))

			// Verify Kubernetes Token Review is configured
			kubernetesTokenReview := authMethod["kubernetes-user"].KubernetesTokenReview
			Expect(kubernetesTokenReview).NotTo(BeNil())

			return nil
		}, 10*time.Second, 2*time.Second).Should(Succeed())
	})

	// Custom matchers (gomega)
	It("should create an AuthorizationPolicy when a ConfigMap is created", func(ctx context.Context) {
		Eventually(func() error {
			createdAuthPolicy := &istiosecurityv1beta1.AuthorizationPolicy{}
			err := envTest.Client.Get(ctx, types.NamespacedName{
				Name:      resourceName,
				Namespace: testNamespaceName,
			}, createdAuthPolicy)

			if err != nil {
				return err
			}

			Expect(createdAuthPolicy.ObjectMeta.OwnerReferences).To(HaveLen(1))
			ownerRef := createdAuthPolicy.ObjectMeta.OwnerReferences[0]
			checkOwnerRef(ownerRef, *createdCfgMap)

			Expect(createdAuthPolicy.Spec.GetAction()).To(Equal(v1beta1.AuthorizationPolicy_CUSTOM))

			return nil
		}, 10*time.Second, 2*time.Second).Should(Succeed())
	})

	It("should create a PeerAuthentication when a ConfigMap is created", func(ctx context.Context) {
		Eventually(func() error {
			createdPeerAuth := &istiosecurityv1beta1.PeerAuthentication{}
			err := envTest.Client.Get(ctx, types.NamespacedName{
				Name:      resourceName,
				Namespace: testNamespaceName,
			}, createdPeerAuth)

			if err != nil {
				return err
			}

			Expect(createdPeerAuth.ObjectMeta.OwnerReferences).To(HaveLen(1))
			ownerRef := createdPeerAuth.ObjectMeta.OwnerReferences[0]
			checkOwnerRef(ownerRef, *createdCfgMap)

			Expect(createdPeerAuth.Spec.GetMtls().GetMode()).To(Equal(v1beta1.PeerAuthentication_MutualTLS_PERMISSIVE))

			return nil
		}, 10*time.Second, 2*time.Second).Should(Succeed())
	})

	// TODO:
	PIt("should have ownerReference on all created auth resources", func(ctx context.Context) {
		// get by owner name label

	})
})

func checkOwnerRef(owner metav1.OwnerReference, cfgMap corev1.ConfigMap) {
	Expect(owner.Name).To(Equal(cfgMap.Name))
	Expect(owner.UID).To(Equal(cfgMap.UID))
	Expect(owner.BlockOwnerDeletion).To(BeNil())
}
