package k8s

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const groupsAnnotation = "platform.flanksource.com/restrict-to-groups"

var log = logf.Log.WithName("ingress-annotator")

type IngressAnnotator struct {
	k8s          client.Client
	svcName      string
	svcNamespace string
	domain       string
}

func NewIngressAnnotator(k8s client.Client, svcName, svcNamespace, domain string) *IngressAnnotator {
	annotator := &IngressAnnotator{
		k8s:          k8s,
		svcName:      svcName,
		svcNamespace: svcNamespace,
		domain:       domain,
	}
	return annotator
}

// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch

func (i *IngressAnnotator) Annotate(ctx context.Context, ingress *v1beta1.Ingress) (*v1beta1.Ingress, bool, error) {
	groups, found := ingress.ObjectMeta.Annotations[groupsAnnotation]
	if !found || groups == "" {
		return nil, false, nil
	}

	svc := &v1.Service{}
	if err := i.k8s.Get(ctx, types.NamespacedName{Name: i.svcName, Namespace: i.svcNamespace}, svc); err != nil {
		return nil, false, errors.Wrapf(err, "failed to list service %s in namespace %s", i.svcName, i.svcNamespace)
	}

	svcIP := svc.Spec.ClusterIP
	if svcIP == "" {
		log.Error(nil, "Service does not have cluster IP", "service", i.svcName, "namespace", i.svcNamespace)
		return nil, false, nil
	}

	newIngress := ingress.DeepCopy()
	newIngress.ObjectMeta.Annotations["nginx.ingress.kubernetes.io/backend-protocol"] = "HTTPS"
	newIngress.ObjectMeta.Annotations["nginx.ingress.kubernetes.io/force-ssl-redirect"] = "true"
	newIngress.ObjectMeta.Annotations["nginx.ingress.kubernetes.io/auth-url"] = fmt.Sprintf("http://%s:4180/oauth2/auth", svcIP)
	newIngress.ObjectMeta.Annotations["nginx.ingress.kubernetes.io/auth-signin"] = fmt.Sprintf("https://oauth2.%s/oauth2/start?rd=https://$host$request_uri$is_args$args", i.domain)
	newIngress.ObjectMeta.Annotations["nginx.ingress.kubernetes.io/auth-response-headers"] = "x-auth-request-user, x-auth-request-email, authorization"
	newIngress.ObjectMeta.Annotations["nginx.ingress.kubernetes.io/configuration-snippet"] = i.configurationSnippet(groups)

	equal := reflect.DeepEqual(ingress.ObjectMeta.Annotations, newIngress.ObjectMeta.Annotations)

	return newIngress, !equal, nil
}

func (i *IngressAnnotator) CS(groupsList string) string {
	return i.configurationSnippet(groupsList)
}

func (i *IngressAnnotator) configurationSnippet(groupsList string) string {
	groups := strings.Split(groupsList, ";")
	snippet := `
auth_request_set $authHeader0 $upstream_http_x_auth_request_user;
proxy_set_header 'x-auth-request-user' $authHeader0;
auth_request_set $authHeader1 $upstream_http_x_auth_request_email;
proxy_set_header 'x-auth-request-email' $authHeader1;
auth_request_set $authHeader2 $upstream_http_authorization;
proxy_set_header 'authorization' $authHeader2;

access_by_lua_block {
	local authorizedGroups = { %s }
	local oauth2GroupAccess = require "oauth2_group_access"

	oauth2GroupAccess:verify_authorization(ngx.var.authHeader2, authorizedGroups)
}
`
	escapedGroups := make([]string, len(groups))
	for i := range groups {
		escapedGroups[i] = "\"" + groups[i] + "\""
	}
	groupsTemplate := strings.Join(escapedGroups, ", ")
	result := fmt.Sprintf(snippet, groupsTemplate)
	return result
}
