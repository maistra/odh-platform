/*
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

//nolint:ireturn //reason: we want to return spi.Interfaces
package resource

import (
	"bytes"
	"context"
	_ "embed" // needed for go:embed directive
	"fmt"
	"net/url"
	"strings"
	"text/template"

	authorinov1beta2 "github.com/kuadrant/authorino/api/v1beta2"
	"github.com/opendatahub-io/odh-platform/pkg/schema"
	"github.com/opendatahub-io/odh-platform/pkg/spi"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//go:embed template/authconfig_anonymous.yaml
var authConfigTemplateAnonymous []byte

//go:embed template/authconfig_userdefined.yaml
var authConfigTemplateUserDefined []byte

type staticTemplateLoader struct {
	audience []string
}

func NewStaticTemplateLoader(audience []string) spi.AuthConfigTemplateLoader {
	return &staticTemplateLoader{audience: audience}
}

func (s *staticTemplateLoader) Load(_ context.Context, authType spi.AuthType, key types.NamespacedName) (authorinov1beta2.AuthConfig, error) {
	authConfig := authorinov1beta2.AuthConfig{}

	templateData := map[string]interface{}{
		"Namespace": key.Namespace,
		"Audiences": s.audience,
	}

	templateContent := authConfigTemplateAnonymous
	if authType == spi.UserDefined {
		templateContent = authConfigTemplateUserDefined
	}

	resolvedTemplate, err := s.resolveTemplate(templateContent, templateData)
	if err != nil {
		return authConfig, fmt.Errorf("could not resolve auth template: %w", err)
	}

	err = schema.ConvertToStructuredResource(resolvedTemplate, &authConfig)
	if err != nil {
		return authConfig, fmt.Errorf("could not load auth template: %w", err)
	}

	return authConfig, nil
}

func (s *staticTemplateLoader) resolveTemplate(tmpl []byte, data map[string]interface{}) ([]byte, error) {
	engine, err := template.New("authconfig").Parse(string(tmpl))
	if err != nil {
		return []byte{}, fmt.Errorf("could not create template engine: %w", err)
	}

	buf := new(bytes.Buffer)

	err = engine.Execute(buf, data)
	if err != nil {
		return []byte{}, fmt.Errorf("could not execute template: %w", err)
	}

	return buf.Bytes(), nil
}

type configMapTemplateLoader struct {
	client   client.Client
	fallback spi.AuthConfigTemplateLoader
}

func NewConfigMapTemplateLoader(cli client.Client, fallback spi.AuthConfigTemplateLoader) spi.AuthConfigTemplateLoader {
	return &configMapTemplateLoader{
		client:   cli,
		fallback: fallback,
	}
}

// TODO: check "authconfig-template" CM in key.Namespace to see if there is a "spec" to use, construct a AuthConfig object
// https://issues.redhat.com/browse/RHOAIENG-847
func (c *configMapTemplateLoader) Load(ctx context.Context, authType spi.AuthType, key types.NamespacedName) (authorinov1beta2.AuthConfig, error) {
	// else
	ac, err := c.fallback.Load(ctx, authType, key)
	if err != nil {
		return authorinov1beta2.AuthConfig{}, fmt.Errorf("could not load from fallback: %w", err)
	}

	return ac, nil
}

type annotationAuthTypeDetector struct {
	annotation string
}

func NewAnnotationAuthTypeDetector(annotation string) spi.AuthTypeDetector {
	return &annotationAuthTypeDetector{
		annotation: annotation,
	}
}

func (k *annotationAuthTypeDetector) Detect(_ context.Context, res *unstructured.Unstructured) (spi.AuthType, error) {
	// TODO: review controllers as package for consts
	if value, exist := res.GetAnnotations()[k.annotation]; exist {
		if strings.EqualFold(value, "true") {
			return spi.UserDefined, nil
		}
	}

	return spi.Anonymous, nil
}

type expressionHostExtractor struct {
	paths []string
}

func NewExpressionHostExtractor(paths []string) spi.HostExtractor {
	return &expressionHostExtractor{paths: paths}
}

func (k *expressionHostExtractor) Extract(target *unstructured.Unstructured) []string {
	hosts := []string{}

	for _, path := range k.paths {
		splitPath := strings.Split(path, ".")

		if foundHost, foundStr, errNestedStr := unstructured.NestedString(target.Object, splitPath...); errNestedStr == nil && foundStr {
			hosts = appendHosts(hosts, foundHost)
		}

		if foundHosts, foundSlice, errNestedSlice := unstructured.NestedStringSlice(target.Object, splitPath...); errNestedSlice == nil && foundSlice {
			hosts = appendHosts(hosts, foundHosts...)
		}
	}

	u := unique(hosts)
	if len(u) == 0 {
		return []string{"unknown.host.com"}
	}

	return u
}

func unique(in []string) []string {
	store := map[string]bool{}

	for _, v := range in {
		store[v] = true
	}

	keys := make([]string, len(store))

	i := 0

	for v := range store {
		keys[i] = v
		i++
	}

	return keys
}

func appendHosts(hosts []string, foundHosts ...string) []string {
	for _, foundHost := range foundHosts {
		if isURI(foundHost) {
			parsedURL, errParse := url.Parse(foundHost)
			if errParse == nil {
				hosts = append(hosts, parsedURL.Host)
			}
		} else {
			hosts = append(hosts, foundHost)
		}
	}

	return hosts
}

func isURI(host string) bool {
	return strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://")
}
