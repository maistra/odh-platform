package env

import (
	"fmt"
	"os"
	"strings"
)

const (
	AuthAudience           = "AUTH_AUDIENCE"
	AuthProvider           = "AUTH_PROVIDER"
	AuthorinoLabelSelector = "AUTHORINO_LABEL"
	ConfigCapabilities     = "CONFIG_CAPABILITIES"
)

func GetAuthorinoLabel() (key, value string, err error) { //nolint:nonamedreturns //reason: having named key-value makes the function easier to understand
	label := getEnvOr(AuthorinoLabelSelector, "security.opendatahub.io/authorization-group=default")
	keyValue := strings.Split(label, "=")

	if len(keyValue) != 2 {
		return "", "", fmt.Errorf("expected authorino label to be in key=value format, got [%s]", label)
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

func GetConfigFile() string {
	return getEnvOr(ConfigCapabilities, "/tmp/capabilities")
}

func getEnvOr(key, defaultValue string) string {
	if env, defined := os.LookupEnv(key); defined {
		return env
	}

	return defaultValue
}
