package controllers

import (
	"context"
	"sort"

	authorinov1beta2 "github.com/kuadrant/authorino/api/v1beta2"
	openshiftv1 "github.com/openshift/api/route/v1"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
)

func (r *PlatformRouteReconciler) reconcileAuthConfig(ctx context.Context, route *openshiftv1.Route) error {

	service := &v1.Service{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: route.Namespace,
		Name:      route.Spec.To.Name},
		service)
	if err != nil {
		return err
	}

	authConfig := &authorinov1beta2.AuthConfig{}
	err = r.Get(ctx, types.NamespacedName{
		Namespace: service.Namespace,
		Name:      service.Name,
	}, authConfig)
	if apierrs.IsNotFound(err) { // TOOD: Not Found yet.. evnetual consistency from other Controller creating the AuthConfig. return non err Result{duration?}
		return err
	} else if err != nil {
		return err
	}

	if !hostConfigured(route.Spec.Host, authConfig.Spec.Hosts) {
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := r.Get(ctx, types.NamespacedName{
				Name:      service.Name,
				Namespace: service.Namespace,
			}, authConfig); err != nil {
				return errors.Wrapf(err, "failed getting AuthConfig %s in namespace %s", service.Name, service.Namespace)
			}

			authConfig.Spec.Hosts = configureHost(route.Spec.Host, authConfig.Spec.Hosts)

			return errors.Wrap(r.Update(ctx, authConfig), "failed updating AuthConfig")
		}); err != nil {

			return errors.Wrap(err, "unable to reconcile the Authorino AuthConfig")
		}
	}

	return nil
}

func hostConfigured(host string, hosts []string) bool {
	for _, h := range hosts {
		if h == host {
			return true
		}
	}
	return false
}

func configureHost(host string, hosts []string) []string {
	res := hosts
	if !hostConfigured(host, res) {
		res = append(res, host)
	}
	sort.Strings(res)
	return res
}
