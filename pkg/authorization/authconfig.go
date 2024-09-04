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

package authorization

import (
	"bytes"
	"context"
	_ "embed" // needed for go:embed directive
	"fmt"
	"strings"
	"text/template"

	authorinov1beta2 "github.com/kuadrant/authorino/api/v1beta2"
	"github.com/opendatahub-io/odh-platform/pkg/schema"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//go:embed template/authconfig_anonymous.yaml
var authConfigTemplateAnonymous []byte

//go:embed template/authconfig_userdefined.yaml
var authConfigTemplateUserDefined []byte

type staticTemplateLoader struct {
}

var _ AuthConfigTemplateLoader = (*staticTemplateLoader)(nil)

func NewStaticTemplateLoader() *staticTemplateLoader {
	return &staticTemplateLoader{}
}

func (s *staticTemplateLoader) Load(_ context.Context, authType AuthType, key types.NamespacedName, templateData map[string]any) (authorinov1beta2.AuthConfig, error) {
	authConfig := authorinov1beta2.AuthConfig{}

	templateContent := authConfigTemplateAnonymous
	if authType == UserDefined {
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

func (s *staticTemplateLoader) resolveTemplate(tmpl []byte, data map[string]any) ([]byte, error) {
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
	fallback AuthConfigTemplateLoader
}

var _ AuthConfigTemplateLoader = (*configMapTemplateLoader)(nil)

func NewConfigMapTemplateLoader(cli client.Client, fallback AuthConfigTemplateLoader) *configMapTemplateLoader {
	return &configMapTemplateLoader{
		client:   cli,
		fallback: fallback,
	}
}

// TODO: check "authconfig-template" CM in key.Namespace to see if there is a "spec" to use, construct a AuthConfig object
// https://issues.redhat.com/browse/RHOAIENG-847
func (c *configMapTemplateLoader) Load(ctx context.Context, authType AuthType, key types.NamespacedName, templateData map[string]any) (authorinov1beta2.AuthConfig, error) {
	// else
	ac, err := c.fallback.Load(ctx, authType, key, templateData)
	if err != nil {
		return authorinov1beta2.AuthConfig{}, fmt.Errorf("could not load from fallback: %w", err)
	}

	return ac, nil
}

type annotationAuthTypeDetector struct {
	annotation string
}

var _ AuthTypeDetector = (*annotationAuthTypeDetector)(nil)

func NewAnnotationAuthTypeDetector(annotation string) *annotationAuthTypeDetector {
	return &annotationAuthTypeDetector{
		annotation: annotation,
	}
}

func (k *annotationAuthTypeDetector) Detect(_ context.Context, res *unstructured.Unstructured) (AuthType, error) {
	// TODO: review controllers as package for consts
	if value, exist := res.GetAnnotations()[k.annotation]; exist {
		if strings.EqualFold(value, "true") {
			return UserDefined, nil
		}
	}

	return Anonymous, nil
}
