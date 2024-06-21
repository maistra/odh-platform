package authorization

import (
	"context"
	"reflect"

	"github.com/opendatahub-io/odh-platform/pkg/env"
	"github.com/opendatahub-io/odh-platform/pkg/label"
	"github.com/pkg/errors"
	"istio.io/api/security/v1beta1"
	istiotypev1beta1 "istio.io/api/type/v1beta1"
	istiosecv1beta1 "istio.io/client-go/pkg/apis/security/v1beta1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
)

func (r *PlatformAuthorizationReconciler) reconcileAuthPolicy(ctx context.Context, target *unstructured.Unstructured) error {
	desired := createAuthorizationPolicy(r.authComponent.Ports, r.authComponent.WorkloadSelector, target)
	found := &istiosecv1beta1.AuthorizationPolicy{}
	justCreated := false

	err := r.Get(ctx, types.NamespacedName{
		Name:      desired.Name,
		Namespace: desired.Namespace,
	}, found)
	if err != nil {
		if apierrs.IsNotFound(err) {
			err = r.Create(ctx, desired)
			if err != nil && !apierrs.IsAlreadyExists(err) {
				return errors.Wrap(err, "unable to create AuthorizationPolicy")
			}

			justCreated = true
		} else {
			return errors.Wrap(err, "unable to fetch AuthorizationPolicy")
		}
	}

	// Reconcile the Istio AuthorizationPolicy if it has been manually modified
	if !justCreated && !CompareAuthPolicies(desired, found) {
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := r.Get(ctx, types.NamespacedName{
				Name:      desired.Name,
				Namespace: desired.Namespace,
			}, found); err != nil {
				return errors.Wrapf(err, "failed getting AuthorizationPolicy %s in namespace %s", desired.Name, desired.Namespace)
			}

			found.Spec = *desired.Spec.DeepCopy()
			found.ObjectMeta.Labels = desired.ObjectMeta.Labels

			return errors.Wrap(r.Update(ctx, found), "failed updating AuthorizationPolicy")
		}); err != nil {
			return errors.Wrap(err, "unable to reconcile the AuthorizationPolicy")
		}
	}

	return nil
}

func createAuthorizationPolicy(ports []string, workloadSelector map[string]string, target *unstructured.Unstructured) *istiosecv1beta1.AuthorizationPolicy {
	policy := &istiosecv1beta1.AuthorizationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:        target.GetName(),
			Namespace:   target.GetNamespace(),
			Labels:      label.ApplyStandard(target.GetLabels()),
			Annotations: map[string]string{},
			OwnerReferences: []metav1.OwnerReference{
				targetToOwnerRef(target),
			},
		},
		Spec: v1beta1.AuthorizationPolicy{
			Selector: &istiotypev1beta1.WorkloadSelector{
				MatchLabels: workloadSelector,
			},
			Action: v1beta1.AuthorizationPolicy_CUSTOM,
			ActionDetail: &v1beta1.AuthorizationPolicy_Provider{
				Provider: &v1beta1.AuthorizationPolicy_ExtensionProvider{
					Name: env.GetAuthProvider(),
				},
			},
		},
	}

	for _, port := range ports {
		rule := v1beta1.Rule{
			To: []*v1beta1.Rule_To{
				{
					Operation: &v1beta1.Operation{
						Ports: []string{port},
						NotPaths: []string{ // TODO: part of AuthRule?
							"/healthz",
							"/debug/pprof/",
							"/metrics",
							"/wait-for-drain",
						},
					},
				},
			},
		}
		policy.Spec.Rules = append(policy.Spec.Rules, &rule)
	}

	return policy
}

func CompareAuthPolicies(authzPolicy1, authzPolicy2 *istiosecv1beta1.AuthorizationPolicy) bool {
	// .Spec contains MessageState from protobuf which has pragma.DoNotCopy (empty mutex slice)
	// go vet complains about copying mutex when calling DeepEquals on passed variables, when it tries to access it.
	// DeepCopy-ing solves this problem as it's using proto.Clone underneath. This implementation recreates mutex instead of
	// directly copying.
	// Alternatively we could break DeepEquals calls to individual .Spec exported fields, but that might mean ensuring we
	// always compare all relevant fields when API changes.
	authzSpec1 := authzPolicy1.Spec.DeepCopy()
	authzSpec2 := authzPolicy2.Spec.DeepCopy()

	return reflect.DeepEqual(authzPolicy1.ObjectMeta.Labels, authzPolicy2.ObjectMeta.Labels) &&
		reflect.DeepEqual(authzSpec1, authzSpec2)
}
