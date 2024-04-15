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

package resources

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

func (s *staticTemplateLoader) Load(ctx context.Context, authType spi.AuthType, key types.NamespacedName) (authorinov1beta2.AuthConfig, error) {
	authConfig := authorinov1beta2.AuthConfig{}

	templateData := map[string]interface{}{
		"Namespace": key.Namespace,
		"Audiences": s.audience,
	}

	template := authConfigTemplateAnonymous
	if authType == spi.UserDefined {
		template = authConfigTemplateUserDefined
	}

	resolvedTemplate, err := s.resolveTemplate(template, templateData)
	if err != nil {
		return authConfig, fmt.Errorf("could not resovle auth template. cause %w", err)
	}
	err = schema.ConvertToStructuredResource(resolvedTemplate, &authConfig)
	if err != nil {
		return authConfig, fmt.Errorf("could not load auth template. cause %w", err)
	}
	return authConfig, nil
}

func (s *staticTemplateLoader) resolveTemplate(tmpl []byte, data map[string]interface{}) ([]byte, error) {
	engine, err := template.New("authconfig").Parse(string(tmpl))
	if err != nil {
		return []byte{}, err
	}
	buf := new(bytes.Buffer)
	err = engine.Execute(buf, data)
	if err != nil {
		return []byte{}, err
	}
	return buf.Bytes(), nil

}

type configMapTemplateLoader struct {
	client   client.Client
	fallback spi.AuthConfigTemplateLoader
}

func NewConfigMapTemplateLoader(client client.Client, fallback spi.AuthConfigTemplateLoader) spi.AuthConfigTemplateLoader {
	return &configMapTemplateLoader{
		client:   client,
		fallback: fallback,
	}
}

func (c *configMapTemplateLoader) Load(ctx context.Context, authType spi.AuthType, key types.NamespacedName) (authorinov1beta2.AuthConfig, error) {
	// TOOD: check "authconfig-template" CM in key.Namespace to see if there is a "spec" to use, construct a AuthConfig object
	// https://issues.redhat.com/browse/RHOAIENG-847

	// else
	return c.fallback.Load(ctx, authType, key)
}

type annotationAuthTypeDetector struct {
	annotation string
}

func NewAnnotationAuthTypeDetector(annotation string) spi.AuthTypeDetector {
	return &annotationAuthTypeDetector{
		annotation: annotation,
	}
}

func (k *annotationAuthTypeDetector) Detect(ctx context.Context, res *unstructured.Unstructured) (spi.AuthType, error) {
	// TODO: review controllers as package for consts
	if value, exist := res.GetAnnotations()[k.annotation]; exist {
		if strings.ToLower(value) == "true" {
			return spi.UserDefined, nil
		}
	}
	return spi.Anonymous, nil
}

type expressionHostExtractor struct {
	paths []string
}

func NewExpressionHostExtractor(paths []string) spi.HostExtractor {
	return &expressionHostExtractor{}
}

func (k *expressionHostExtractor) Extract(target *unstructured.Unstructured) []string {
	hosts := []string{}

	for _, path := range k.paths {
		host, found, err := unstructured.NestedString(target.Object, strings.Split(path, ".")...)
		if err == nil && found {
			// TODO log err?
			url, err := url.Parse(host)
			if err == nil {
				hosts = append(hosts, url.Host)
			}
		}
	}

	return unique(hosts)
}

func unique(in []string) []string {
	m := map[string]bool{}
	for _, v := range in {
		m[v] = true
	}
	k := make([]string, len(m))
	i := 0
	for v := range m {
		k[i] = v
		i++
	}
	return k
}
