package controllers

import (
	"context"
	"reflect"

	"github.com/pkg/errors"
	"istio.io/api/security/v1beta1"
	istiotypev1beta1 "istio.io/api/type/v1beta1"
	istiosecv1beta1 "istio.io/client-go/pkg/apis/security/v1beta1"
	v1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
)

func (r *PlatformAuthorizationReconciler) reconcileAuthPolicy(ctx context.Context, service *v1.Service) error {

	desired := createAuthorizationPolicy(service)
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
			// TODO: Merge Annotations?

			return errors.Wrap(r.Update(ctx, found), "failed updating AuthorizationPolicy")
		}); err != nil {

			return errors.Wrap(err, "unable to reconcile the AuthorizationPolicy")
		}
	}

	return nil
}

// TODO: Owned by?
func createAuthorizationPolicy(service *v1.Service) *istiosecv1beta1.AuthorizationPolicy {
	policy := &istiosecv1beta1.AuthorizationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:        service.Name,
			Namespace:   service.Namespace,
			Labels:      service.Labels,      // TODO: Where to fetch lables from
			Annotations: map[string]string{}, // TODO: where to fetch annotations from? part-of "service comp" or "platform?"
			OwnerReferences: []metav1.OwnerReference{
				serviceToOwnerRef(service),
			},
		},
		Spec: v1beta1.AuthorizationPolicy{
			Selector: &istiotypev1beta1.WorkloadSelector{
				MatchLabels: service.Spec.Selector,
			},
			Action: v1beta1.AuthorizationPolicy_CUSTOM,
			ActionDetail: &v1beta1.AuthorizationPolicy_Provider{
				Provider: &v1beta1.AuthorizationPolicy_ExtensionProvider{
					Name: "opendatahub-auth-provider", // TODO: Make configurable
				},
			},
		},
	}
	for _, port := range service.Spec.Ports {
		rule := v1beta1.Rule{
			To: []*v1beta1.Rule_To{
				{
					Operation: &v1beta1.Operation{
						Ports: []string{port.TargetPort.String()}, // TODO: TargetPort could be a port name, does that work or does it need to be resolved?
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

func CompareAuthPolicies(m1, m2 *istiosecv1beta1.AuthorizationPolicy) bool {
	return reflect.DeepEqual(m1.ObjectMeta.Labels, m2.ObjectMeta.Labels) &&
		reflect.DeepEqual(m1.Spec, m2.Spec)
}
