/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ingress

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	perrors "github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	groupsAnnotation          = "platform.flanksource.com/restrict-to-groups"
	extraSnippetAnnotation    = "platform.flanksource.com/extra-configuration-snippet"
	passAuthHeadersAnnotation = "platform.flanksource.com/pass-auth-headers"
	oauthSnippet              = `
auth_request_set $authHeader0 $upstream_http_x_auth_request_user;
auth_request_set $authHeader1 $upstream_http_x_auth_request_email;
auth_request_set $authHeader2 $upstream_http_authorization;

access_by_lua_block {
	local authorizedGroups = { %s }
	local oauth2GroupAccess = require "oauth2_group_access"

	oauth2GroupAccess:verify_authorization(ngx.var.authHeader2, authorizedGroups)
}
`
	passHeadersSnippet = `
proxy_set_header 'x-auth-request-user' $authHeader0;
proxy_set_header 'x-auth-request-email' $authHeader1;
proxy_set_header 'authorization' $authHeader2;
`
)

type IngressReconciler struct {
	client.Client
	SvcName      string
	SvcNamespace string
	Domain       string
}

func newIngressReconciler(mgr manager.Manager, svcName, svcNamespace, domain string) reconcile.Reconciler {
	return &IngressReconciler{
		Client:       mgr.GetClient(),
		SvcName:      svcName,
		SvcNamespace: svcNamespace,
		Domain:       domain,
	}
}

func addIngressReconciler(mgr manager.Manager, r reconcile.Reconciler) error {
	c, err := controller.New(name, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	if err := c.Watch(
		&source.Kind{Type: &v1beta1.Ingress{}}, &handler.EnqueueRequestForObject{},
		predicate.Funcs{CreateFunc: onCreate, UpdateFunc: onUpdate},
	); err != nil {
		return err
	}
	return nil
}

// +kubebuilder:rbac:groups="extensions",resources=ingresses,verbs=get;list;update;watch
func (r *IngressReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	ingress := &v1beta1.Ingress{}
	if err := r.Get(ctx, request.NamespacedName, ingress); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	updatedIngress, changed, err := r.Annotate(ctx, ingress)
	if err != nil || !changed {
		return reconcile.Result{}, err
	}

	if err := r.Client.Update(ctx, updatedIngress); err != nil {
		log.Error(err, "failed to update", "ingress", updatedIngress.Name, "namespace", updatedIngress.Namespace)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
func (r *IngressReconciler) Annotate(ctx context.Context, ingress *v1beta1.Ingress) (*v1beta1.Ingress, bool, error) {
	groups, found := ingress.ObjectMeta.Annotations[groupsAnnotation]
	if !found || groups == "" {
		return nil, false, nil
	}

	svc := &v1.Service{}
	if err := r.Get(ctx, types.NamespacedName{Name: r.SvcName, Namespace: r.SvcNamespace}, svc); err != nil {
		return nil, false, perrors.Wrapf(err, "failed to list service %s in namespace %s", r.SvcName, r.SvcNamespace)
	}

	svcIP := svc.Spec.ClusterIP
	if svcIP == "" {
		log.Error(nil, "Service does not have cluster IP", "service", r.SvcName, "namespace", r.SvcNamespace)
		return nil, false, nil
	}

	passHeadersStr, found := ingress.ObjectMeta.Annotations[passAuthHeadersAnnotation]
	if !found {
		passHeadersStr = "true"
	}
	passHeaders := passHeadersStr == "true"

	extraSnippet, found := ingress.ObjectMeta.Annotations[extraSnippetAnnotation]
	if !found {
		extraSnippet = ""
	}

	newIngress := ingress.DeepCopy()
	newIngress.ObjectMeta.Annotations["nginx.ingress.kubernetes.io/auth-url"] = fmt.Sprintf("http://%s:4180/oauth2/auth", svcIP)
	newIngress.ObjectMeta.Annotations["nginx.ingress.kubernetes.io/auth-signin"] = fmt.Sprintf("https://oauth2.%s/oauth2/start?rd=https://$host$request_uri$is_args$args", r.Domain)
	newIngress.ObjectMeta.Annotations["nginx.ingress.kubernetes.io/configuration-snippet"] = configurationSnippet(groups, passHeaders, extraSnippet)

	equal := reflect.DeepEqual(ingress.ObjectMeta.Annotations, newIngress.ObjectMeta.Annotations)

	return newIngress, !equal, nil
}

func configurationSnippet(groupsList string, passHeaders bool, extraSnippet string) string {
	groups := strings.Split(groupsList, ";")

	escapedGroups := make([]string, len(groups))
	for i := range groups {
		escapedGroups[i] = "\"" + groups[i] + "\""
	}
	groupsTemplate := strings.Join(escapedGroups, ", ")
	result := fmt.Sprintf(oauthSnippet, groupsTemplate)

	if passHeaders {
		result = result + "\n" + passHeadersSnippet
	}

	if extraSnippet != "" {
		result = result + "\n" + extraSnippet
	}
	return result
}
