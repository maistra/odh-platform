package env

import (
	"os"
	"strings"

	"github.com/pkg/errors"
)

const (
	MeshNamespaceEnv       = "MESH_NAMESPACE"
	ControlPlaneEnv        = "CONTROL_PLANE_NAME"
	AuthAudience           = "AUTH_AUDIENCE"
	AuthProvider           = "AUTH_PROVIDER"
	AuthorinoLabelSelector = "AUTHORINO_LABEL"
)

func getControlPlaneName() string {
	return getEnvOr(ControlPlaneEnv, "basic")
}

func getMeshNamespace() string {
	return getEnvOr(MeshNamespaceEnv, "istio-system")
}

func GetAuthorinoLabel() (string, string, error) {
	label := getEnvOr(AuthorinoLabelSelector, "security.opendatahub.io/authorization-group=default")
	keyValue := strings.Split(label, "=")

	if len(keyValue) != 2 {
		return "", "", errors.Errorf("Expected authorino label to be in key=value format, got [%s]", label)
	}

	return keyValue[0], keyValue[1], nil
}

func GetAuthProvider() string {
	return getEnvOr(AuthProvider, "opendatahub-auth-provider")
}

func GetAuthAudience() []string {
	aud := getEnvOr(AuthAudience, "https://kubernetes.default.svc")
	audiences := strings.Split(aud, ",")

	for i := range audiences {
		audiences[i] = strings.TrimSpace(audiences[i])
	}

	return audiences
}

func getEnvOr(key, defaultValue string) string {
	if env, defined := os.LookupEnv(key); defined {
		return env
	}

	return defaultValue
}
