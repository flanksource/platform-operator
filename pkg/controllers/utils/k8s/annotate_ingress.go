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
const extraSnippetAnnotation = "platform.flanksource.com/extra-configuration-snippet"
const passAuthHeadersAnnotation = "platform.flanksource.com/pass-auth-headers"

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
	newIngress.ObjectMeta.Annotations["nginx.ingress.kubernetes.io/auth-signin"] = fmt.Sprintf("https://oauth2.%s/oauth2/start?rd=https://$host$request_uri$is_args$args", i.domain)
	newIngress.ObjectMeta.Annotations["nginx.ingress.kubernetes.io/configuration-snippet"] = i.configurationSnippet(groups, passHeaders, extraSnippet)

	equal := reflect.DeepEqual(ingress.ObjectMeta.Annotations, newIngress.ObjectMeta.Annotations)

	return newIngress, !equal, nil
}

func (i *IngressAnnotator) configurationSnippet(groupsList string, passHeaders bool, extraSnippet string) string {
	groups := strings.Split(groupsList, ";")
	snippet := `
auth_request_set $authHeader0 $upstream_http_x_auth_request_user;
auth_request_set $authHeader1 $upstream_http_x_auth_request_email;
auth_request_set $authHeader2 $upstream_http_authorization;

access_by_lua_block {
	local authorizedGroups = { %s }
	local oauth2GroupAccess = require "oauth2_group_access"

	oauth2GroupAccess:verify_authorization(ngx.var.authHeader2, authorizedGroups)
}
`

	passHeadersSnippet := `
proxy_set_header 'x-auth-request-user' $authHeader0;
proxy_set_header 'x-auth-request-email' $authHeader1;
proxy_set_header 'authorization' $authHeader2;
`
	escapedGroups := make([]string, len(groups))
	for i := range groups {
		escapedGroups[i] = "\"" + groups[i] + "\""
	}
	groupsTemplate := strings.Join(escapedGroups, ", ")
	result := fmt.Sprintf(snippet, groupsTemplate)

	if passHeaders {
		result = result + "\n" + passHeadersSnippet
	}

	if extraSnippet != "" {
		result = result + "\n" + extraSnippet
	}
	return result
}
