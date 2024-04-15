package schema

import (
	authorino "github.com/kuadrant/authorino/api/v1beta2"
	//routev1 "github.com/openshift/api/route/v1"
	istiosecurity "istio.io/client-go/pkg/apis/security/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

// RegisterSchemes adds schemes of used resources to controller's scheme.
func RegisterSchemes(s *runtime.Scheme) {
	utilruntime.Must(clientgoscheme.AddToScheme(s))
	//utilruntime.Must(routev1.Install(s))
	utilruntime.Must(authorino.AddToScheme(s))
	utilruntime.Must(istiosecurity.AddToScheme(s))
}
