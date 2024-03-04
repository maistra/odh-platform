package controllers

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"

	authorinov1beta2 "github.com/kuadrant/authorino/api/v1beta2"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
)

func (r *PlatformAuthorizationReconciler) reconcileAuthConfig(ctx context.Context, service *v1.Service) error {

	desired, err := createAuthConfig(service)
	if err != nil {
		return errors.Wrap(err, "could not create destired AuthConfig")
	}

	found := &authorinov1beta2.AuthConfig{}
	justCreated := false

	err = r.Get(ctx, types.NamespacedName{
		Name:      desired.Name,
		Namespace: desired.Namespace,
	}, found)
	if err != nil {
		if apierrs.IsNotFound(err) {

			err = r.Create(ctx, desired)
			if err != nil && !apierrs.IsAlreadyExists(err) {

				return errors.Wrap(err, "unable to create AuthConfig")
			}

			justCreated = true
		} else {

			return errors.Wrap(err, "unable to fetch AuthConfig")
		}
	}

	// Reconcile the Authorino AuthConfig if it has been manually modified
	if !justCreated && !CompareAuthConfigs(desired, found) {

		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := r.Get(ctx, types.NamespacedName{
				Name:      desired.Name,
				Namespace: desired.Namespace,
			}, found); err != nil {
				return errors.Wrapf(err, "failed getting AuthConfig %s in namespace %s", desired.Name, desired.Namespace)
			}

			found.Spec = *desired.Spec.DeepCopy()
			found.ObjectMeta.Labels = desired.ObjectMeta.Labels
			// TODO: Merge Annotations?

			return errors.Wrap(r.Update(ctx, found), "failed updating AuthConfig")
		}); err != nil {

			return errors.Wrap(err, "unable to reconcile the Authorino AuthConfig")
		}
	}

	return nil
}

func createAuthConfig(service *v1.Service) (*authorinov1beta2.AuthConfig, error) {
	keyVal, err := getAuthorinoLabel()
	if err != nil {
		return &authorinov1beta2.AuthConfig{}, errors.Wrap(err, "could not get authorino label selcetor")
	}

	labels := service.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	labels[keyVal[0]] = keyVal[1]

	config := &authorinov1beta2.AuthConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:        service.Name,
			Namespace:   service.Namespace,
			Labels:      labels,              // TODO: Where to fetch lables from
			Annotations: map[string]string{}, // TODO: where to fetch annotations from? part-of "service comp" or "platform?"
		},
		// TODO: Impl OwnerRef:  Assume Service for now
		Spec: authorinov1beta2.AuthConfigSpec{
			Hosts: []string{
				service.Name,
				service.Name + ".svc",
				service.Name + ".svc.cluster.local",
			},
		},
	}
	if strings.ToLower(service.Labels[AnnotationAuthEnabled]) != "true" {
		config.Spec.Authentication = map[string]authorinov1beta2.AuthenticationSpec{
			"anonymous": {
				AuthenticationMethodSpec: authorinov1beta2.AuthenticationMethodSpec{
					AnonymousAccess: &authorinov1beta2.AnonymousAccessSpec{},
				},
			},
		}
	} else {
		config.Spec.Authentication = map[string]authorinov1beta2.AuthenticationSpec{
			"kubernetes": {
				AuthenticationMethodSpec: authorinov1beta2.AuthenticationMethodSpec{
					KubernetesTokenReview: &authorinov1beta2.KubernetesTokenReviewSpec{
						Audiences: getAuthAudience(),
					},
				},
			},
		}
		config.Spec.Authorization = map[string]authorinov1beta2.AuthorizationSpec{
			"kubernetes": {
				AuthorizationMethodSpec: authorinov1beta2.AuthorizationMethodSpec{
					KubernetesSubjectAccessReview: &authorinov1beta2.KubernetesSubjectAccessReviewAuthorizationSpec{
						ResourceAttributes: &authorinov1beta2.KubernetesSubjectAccessReviewResourceAttributesSpec{ // TODO: Lookup AuthRule
							Verb:        toValue("get"),
							Group:       toValue(""),
							Resource:    toValue("services"),
							Namespace:   toValue(service.Namespace),
							SubResource: toValue(""),
							Name:        toValue(service.Name),
						},
						User: toSelector("auth.identity.user.username"),
					},
				},
			},
		}
	}
	return config, nil
}

// TODO: We have multiple Controllers adding Spec.Hosts. Compare specifically that the ones we need are in the list, if more assume equal?
func CompareAuthConfigs(m1, m2 *authorinov1beta2.AuthConfig) bool {
	return reflect.DeepEqual(m1.ObjectMeta.Labels, m2.ObjectMeta.Labels) &&
		reflect.DeepEqual(m1.Spec, m2.Spec)
}

func toValue(val string) authorinov1beta2.ValueOrSelector {
	r := runtime.RawExtension{}
	rv, err := json.Marshal(val)
	if err == nil {
		r.Raw = rv
	}
	return authorinov1beta2.ValueOrSelector{Value: r}

}

func toSelector(val string) *authorinov1beta2.ValueOrSelector {
	return &authorinov1beta2.ValueOrSelector{Selector: val}
}
