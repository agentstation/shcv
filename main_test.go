package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

func TestProcessChart(t *testing.T) {
	// Create temporary directory for test chart
	tmpDir, err := os.MkdirTemp("", "helm-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create templates directory
	templatesDir := filepath.Join(tmpDir, "templates")
	err = os.MkdirAll(templatesDir, 0755)
	assert.NoError(t, err)

	// Create test template file
	templateContent := `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: test-ingress
spec:
  rules:
  - host: {{ .Values.gateway.domain | default "api.example.com" }}
    http:
      paths:
      - path: {{ .Values.path | default "/" }}
        backend:
          service:
            name: {{ .Values.service.name }}
            port:
              number: {{ .Values.service.port | default 80 }}
`
	err = os.WriteFile(filepath.Join(templatesDir, "ingress.yaml"), []byte(templateContent), 0644)
	assert.NoError(t, err)

	// Test with verbose flag first to see the success message
	output := captureOutput(func() {
		err := processChart(tmpDir, true)
		assert.NoError(t, err)
	})

	// Verify verbose output
	assert.Contains(t, output, "Found 1 template files")
	assert.Contains(t, output, "Found 4 value references")
	assert.Contains(t, output, "gateway.domain")
	assert.Contains(t, output, "api.example.com")
	assert.Contains(t, output, "Successfully updated")
	assert.Contains(t, output, "values.yaml")

	// Verify values.yaml was created
	data, err := os.ReadFile(filepath.Join(tmpDir, "values.yaml"))
	assert.NoError(t, err)

	var values map[string]any
	err = yaml.Unmarshal(data, &values)
	assert.NoError(t, err)

	gateway, ok := values["gateway"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "api.example.com", gateway["domain"])

	// Test without verbose flag
	err = processChart(tmpDir, false)
	assert.NoError(t, err)
}

func TestProcessChartErrors(t *testing.T) {
	// Test with non-existent directory
	err := processChart("/nonexistent", false)
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "invalid chart directory"))

	// Create empty directory without templates
	tmpDir, err := os.MkdirTemp("", "helm-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	err = processChart(tmpDir, false)
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "templates directory not found"))
}
