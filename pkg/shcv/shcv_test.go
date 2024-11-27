package shcv

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestVersion(t *testing.T) {
	assert.NotEmpty(t, Version)
	assert.Regexp(t, `^\d+\.\d+\.\d+$`, Version)
}

func TestNewChart(t *testing.T) {
	// Create test directories
	tmpDir, err := os.MkdirTemp("", "helm-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Test with default options
	chart, err := NewChart(tmpDir, nil)
	assert.NoError(t, err)
	assert.Equal(t, tmpDir, chart.Dir)
	assert.Equal(t, filepath.Join(tmpDir, "values.yaml"), chart.ValuesFile)

	// Test with custom options
	opts := &Options{
		ValuesFileName: "custom-values.yaml",
		TemplatesDir:   "custom-templates",
		DefaultValues: map[string]string{
			"domain": "custom.example.com",
		},
	}
	chart, err = NewChart(tmpDir, opts)
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(tmpDir, "custom-values.yaml"), chart.ValuesFile)
}

func TestNewChartValidation(t *testing.T) {
	// Test with empty directory
	_, err := NewChart("", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")

	// Test with non-existent directory
	_, err = NewChart("/nonexistent/path", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid chart directory")

	// Test with file instead of directory
	tmpFile, err := os.CreateTemp("", "test-file")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	err = tmpFile.Close()
	assert.NoError(t, err)

	_, err = NewChart(tmpFile.Name(), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is not a directory")
}

func TestChartProcessing(t *testing.T) {
	// Create temporary directory for test chart
	tmpDir, err := os.MkdirTemp("", "helm-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create templates directory
	templatesDir := filepath.Join(tmpDir, "templates")
	err = os.MkdirAll(templatesDir, 0755)
	assert.NoError(t, err)

	// Create test files with various value formats
	files := map[string]string{
		"deployment.yaml": `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Values.name }}
spec:
  replicas: {{ .Values.replicas | default 3 }}
  template:
    spec:
      containers:
      - name: app
        image: {{ .Values.image.repository }}:{{ .Values.image.tag | default 'latest' }}
        ports:
        - containerPort: {{ .Values.containerPort | default 8080 }}`,

		"service.yaml": `
apiVersion: v1
kind: Service
metadata:
  name: {{ .Values.name }}
  annotations:
    prometheus.io/enabled: {{ .Values.monitoring.enabled | default "true" }}
spec:
  ports:
  - port: {{ .Values.service.port | default 80 }}
    targetPort: {{ .Values.service.targetPort | default .Values.containerPort }}`,

		"ingress.yaml": `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{ .Values.name }}
spec:
  rules:
  - host: {{ .Values.ingress.host | default "example.com" }}`,
	}

	for filename, content := range files {
		err = os.WriteFile(filepath.Join(templatesDir, filename), []byte(content), 0644)
		assert.NoError(t, err)
	}

	// Create partial values.yaml
	valuesContent := `
name: my-app
image:
  repository: nginx
service:
  targetPort: 8080
`
	err = os.WriteFile(filepath.Join(tmpDir, "values.yaml"), []byte(valuesContent), 0644)
	assert.NoError(t, err)

	// Initialize and process chart with custom defaults
	opts := &Options{
		DefaultValues: map[string]string{
			"replicas": "3", // Override default value
		},
	}
	chart, err := NewChart(tmpDir, opts)
	assert.NoError(t, err)

	// Test LoadValues
	err = chart.LoadValues()
	assert.NoError(t, err)
	assert.Equal(t, "my-app", chart.Values["name"])

	// Test FindTemplates
	err = chart.FindTemplates()
	assert.NoError(t, err)
	assert.Len(t, chart.Templates, 3)

	// Test ParseTemplates
	err = chart.ParseTemplates()
	assert.NoError(t, err)

	// Verify all references were found
	expectedPaths := map[string]bool{
		"name":               true,
		"replicas":           true,
		"image.repository":   true,
		"image.tag":          true,
		"containerPort":      true,
		"monitoring.enabled": true,
		"service.port":       true,
		"service.targetPort": true,
		"ingress.host":       true,
	}

	foundPaths := make(map[string]bool)
	for _, ref := range chart.References {
		foundPaths[ref.Path] = true
		assert.Greater(t, ref.LineNumber, 0)
		assert.NotEmpty(t, ref.SourceFile)
	}
	assert.Equal(t, expectedPaths, foundPaths)

	// Test UpdateValues
	err = chart.UpdateValues()
	assert.NoError(t, err)
	assert.True(t, chart.Changed)

	// Verify final values
	data, err := os.ReadFile(chart.ValuesFile)
	assert.NoError(t, err)

	var values map[string]any
	err = yaml.Unmarshal(data, &values)
	assert.NoError(t, err)

	// Check that existing values were preserved
	assert.Equal(t, "my-app", values["name"])
	image, ok := values["image"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "nginx", image["repository"])
	assert.Equal(t, "latest", image["tag"])

	// Check that new values were added with defaults
	assert.Equal(t, "3", values["replicas"])
	assert.Equal(t, "8080", values["containerPort"])
	monitoring, ok := values["monitoring"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "true", monitoring["enabled"])
	service, ok := values["service"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "80", service["port"])
	assert.Equal(t, "8080", service["targetPort"])
	ingress, ok := values["ingress"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "example.com", ingress["host"])
}

func TestCustomOptions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "helm-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create custom templates directory
	customTemplatesDir := filepath.Join(tmpDir, "custom-templates")
	err = os.MkdirAll(customTemplatesDir, 0755)
	assert.NoError(t, err)

	// Create test template
	templateContent := `
apiVersion: v1
kind: Service
metadata:
  name: {{ .Values.name }}
spec:
  ports:
  - port: {{ .Values.port }}`

	err = os.WriteFile(filepath.Join(customTemplatesDir, "service.yaml"), []byte(templateContent), 0644)
	assert.NoError(t, err)

	// Test with custom options
	opts := &Options{
		ValuesFileName: "custom-values.yaml",
		TemplatesDir:   "custom-templates",
		DefaultValues: map[string]string{
			"port": "9090",
		},
	}

	chart, err := NewChart(tmpDir, opts)
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(tmpDir, "custom-values.yaml"), chart.ValuesFile)

	err = chart.LoadValues()
	assert.NoError(t, err)

	err = chart.FindTemplates()
	assert.NoError(t, err)
	assert.Len(t, chart.Templates, 1)

	err = chart.ParseTemplates()
	assert.NoError(t, err)

	err = chart.UpdateValues()
	assert.NoError(t, err)

	// Verify custom default value was used
	data, err := os.ReadFile(chart.ValuesFile)
	assert.NoError(t, err)

	var values map[string]any
	err = yaml.Unmarshal(data, &values)
	assert.NoError(t, err)

	assert.Equal(t, "9090", values["port"])
}

func TestNoChangesNeeded(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "helm-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create templates directory
	templatesDir := filepath.Join(tmpDir, "templates")
	err = os.MkdirAll(templatesDir, 0755)
	assert.NoError(t, err)

	// Create test template
	templateContent := `
apiVersion: v1
kind: Service
metadata:
  name: {{ .Values.name }}`

	err = os.WriteFile(filepath.Join(templatesDir, "service.yaml"), []byte(templateContent), 0644)
	assert.NoError(t, err)

	// Create values.yaml with all required values
	valuesContent := `
name: my-service`

	err = os.WriteFile(filepath.Join(tmpDir, "values.yaml"), []byte(valuesContent), 0644)
	assert.NoError(t, err)

	// Process chart
	chart, err := NewChart(tmpDir, nil)
	assert.NoError(t, err)

	err = chart.LoadValues()
	assert.NoError(t, err)

	err = chart.FindTemplates()
	assert.NoError(t, err)

	err = chart.ParseTemplates()
	assert.NoError(t, err)

	err = chart.UpdateValues()
	assert.NoError(t, err)

	// Verify no changes were made
	assert.False(t, chart.Changed)
}
