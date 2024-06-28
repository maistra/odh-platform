package authorization_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/opendatahub-io/odh-platform/controllers/authorization"
	pschema "github.com/opendatahub-io/odh-platform/pkg/schema"
	"github.com/opendatahub-io/odh-platform/pkg/spi"
	"github.com/opendatahub-io/odh-platform/test/labels"
	"go.uber.org/zap/zapcore"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var (
	cli     client.Client
	envTest *envtest.Environment
)

var testScheme = runtime.NewScheme()

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Reconcilers suite")
}

var _ = SynchronizedBeforeSuite(func(ctx context.Context) {
	if !Label(labels.EnvTest).MatchesLabelFilter(GinkgoLabelFilter()) {
		return
	}

	opts := zap.Options{
		Development: true,
		TimeEncoder: zapcore.TimeEncoderOfLayout(time.RFC3339),
	}
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseFlagOptions(&opts)))

	By("Bootstrapping k8s test environment")
	envTest = &envtest.Environment{
		CRDInstallOptions: envtest.CRDInstallOptions{
			Scheme:             testScheme,
			CRDs:               loadCRDs(),
			Paths:              []string{filepath.Join("..", "config", "crd", "external")},
			ErrorIfPathMissing: true,
			CleanUpAfterUse:    false,
		},
	}

	cfg, err := envTest.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	pschema.RegisterSchemes(testScheme)
	utilruntime.Must(v1.AddToScheme(testScheme))

	cli, err = client.New(cfg, client.Options{Scheme: testScheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(cli).NotTo(BeNil())

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:         testScheme,
		LeaderElection: false,
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
	})
	Expect(err).NotTo(HaveOccurred())

	err = authorization.NewPlatformAuthorizationReconciler(
		cli,
		ctrl.Log.WithName("controllers").WithName("platform"),
		spi.AuthorizationComponent{
			CustomResourceType: spi.ResourceSchema{
				GroupVersionKind: schema.GroupVersionKind{Version: "v1", Kind: "service"},
				Resources:        "services",
			},
			WorkloadSelector: map[string]string{},
			Ports:            []string{},
			HostPaths:        []string{"status.url"},
		},
	).SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(ctx)).To(Succeed(), "Failed to start manager")
	}()
}, func() {})

var _ = SynchronizedAfterSuite(func() {}, func() {
	if !Label(labels.EnvTest).MatchesLabelFilter(GinkgoLabelFilter()) {
		return
	}
	By("Tearing down the test environment")
	Expect(envTest.Stop()).To(Succeed())
})

func loadCRDs() []*v1.CustomResourceDefinition {
	return []*v1.CustomResourceDefinition{}
}
