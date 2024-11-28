package shcv

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// Helper function to sort ValueRefs by Path for consistent comparison
func sortValueRefs(refs []ValueRef) {
	sort.Slice(refs, func(i, j int) bool {
		return refs[i].Path < refs[j].Path
	})
}

func TestVersion(t *testing.T) {
	t.Run("version format", func(t *testing.T) {
		assert.NotEmpty(t, Version)
		assert.Regexp(t, `^\d+\.\d+\.\d+$`, Version)
	})
}

func TestOptions(t *testing.T) {
	t.Run("default options", func(t *testing.T) {
		opts := DefaultOptions()
		assert.Equal(t, "values.yaml", opts.ValuesFileName)
		assert.Equal(t, "templates", opts.TemplatesDir)
		assert.NotNil(t, opts.DefaultValues)
		assert.Equal(t, "api.example.com", opts.DefaultValues["domain"])
	})

	t.Run("merge with default options", func(t *testing.T) {
		customOpts := &Options{
			ValuesFileName: "custom.yaml",
			DefaultValues: map[string]string{
				"custom": "value",
			},
		}
		chart, err := NewChart(".", customOpts)
		require.NoError(t, err)
		assert.Equal(t, "custom.yaml", chart.options.ValuesFileName)
		assert.Equal(t, "templates", chart.options.TemplatesDir)
		assert.Equal(t, "value", chart.options.DefaultValues["custom"])
	})
}

func TestNewChart(t *testing.T) {
	t.Run("invalid inputs", func(t *testing.T) {
		tests := []struct {
			name    string
			dir     string
			wantErr string
		}{
			{"empty dir", "", "cannot be empty"},
			{"non-existent dir", "/nonexistent/path", "invalid chart directory"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := NewChart(tt.dir, nil)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			})
		}
	})

	t.Run("file instead of directory", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test-file")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		_, err = NewChart(tmpFile.Name(), nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "is not a directory")
	})

	t.Run("valid directory", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "helm-test")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		chart, err := NewChart(tmpDir, nil)
		assert.NoError(t, err)
		assert.Equal(t, tmpDir, chart.Dir)
		assert.Equal(t, filepath.Join(tmpDir, "values.yaml"), chart.ValuesFile)
	})
}

func TestValueParsing(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "helm-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	templatesDir := filepath.Join(tmpDir, "templates")
	require.NoError(t, os.MkdirAll(templatesDir, 0755))

	tests := []struct {
		name     string
		template string
		want     []ValueRef
	}{
		{
			name: "simple value",
			template: `
apiVersion: v1
kind: Service
metadata:
  name: {{ .Values.name }}`,
			want: []ValueRef{{
				Path:       "name",
				LineNumber: 5,
			}},
		},
		{
			name: "value with double-quoted default",
			template: `
metadata:
  name: {{ .Values.name | default "example" }}`,
			want: []ValueRef{{
				Path:         "name",
				DefaultValue: "example",
				LineNumber:   3,
			}},
		},
		{
			name: "value with single-quoted default",
			template: `
metadata:
  name: {{ .Values.name | default 'example' }}`,
			want: []ValueRef{{
				Path:         "name",
				DefaultValue: "example",
				LineNumber:   3,
			}},
		},
		{
			name: "value with numeric default",
			template: `
spec:
  replicas: {{ .Values.replicas | default 3 }}`,
			want: []ValueRef{{
				Path:         "replicas",
				DefaultValue: "3",
				LineNumber:   3,
			}},
		},
		{
			name: "nested values",
			template: `
spec:
  image: {{ .Values.image.repository }}:{{ .Values.image.tag | default "latest" }}`,
			want: []ValueRef{
				{
					Path:       "image.repository",
					LineNumber: 3,
				},
				{
					Path:         "image.tag",
					DefaultValue: "latest",
					LineNumber:   3,
				},
			},
		},
		{
			name: "multiple defaults for same value",
			template: `
metadata:
  domain: {{ .Values.domain | default "dev.example.com" }}
spec:
  url: {{ .Values.domain | default "stage.example.com" }}`,
			want: []ValueRef{
				{
					Path:         "domain",
					DefaultValue: "dev.example.com",
					LineNumber:   3,
				},
				{
					Path:         "domain",
					DefaultValue: "stage.example.com",
					LineNumber:   5,
				},
			},
		},
		{
			name: "multiple defaults in same line",
			template: `
spec:
  urls: 
    - {{ .Values.domain | default "dev.example.com" }}/api
    - {{ .Values.domain | default "stage.example.com" }}/auth`,
			want: []ValueRef{
				{
					Path:         "domain",
					DefaultValue: "dev.example.com",
					LineNumber:   4,
				},
				{
					Path:         "domain",
					DefaultValue: "stage.example.com",
					LineNumber:   5,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			templateFile := filepath.Join(templatesDir, tt.name+".yaml")
			err := os.WriteFile(templateFile, []byte(tt.template), 0644)
			require.NoError(t, err)

			chart, err := NewChart(tmpDir, nil)
			require.NoError(t, err)

			err = chart.FindTemplates()
			require.NoError(t, err)

			err = chart.ParseTemplates()
			require.NoError(t, err)

			// Filter references for this template
			var refs []ValueRef
			for _, ref := range chart.References {
				if filepath.Base(ref.SourceFile) == tt.name+".yaml" {
					refs = append(refs, ValueRef{
						Path:         ref.Path,
						DefaultValue: ref.DefaultValue,
						LineNumber:   ref.LineNumber,
					})
				}
			}

			// Sort both slices for consistent comparison
			sortValueRefs(refs)
			sortValueRefs(tt.want)
			assert.Equal(t, tt.want, refs)
		})
	}
}

func TestDefaultValues(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "helm-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	templatesDir := filepath.Join(tmpDir, "templates")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(templatesDir, 0755))

	// Create a template with multiple default values
	templateContent := `
apiVersion: v1
kind: Service
metadata:
  domain: {{ .Values.domain | default "dev.example.com" }}
spec:
  url: {{ .Values.domain | default "stage.example.com" }}`

	err = os.WriteFile(filepath.Join(templatesDir, "service.yaml"), []byte(templateContent), 0644)
	require.NoError(t, err)

	chart, err := NewChart(tmpDir, nil)
	require.NoError(t, err)

	t.Log("Finding templates...")
	err = chart.FindTemplates()
	require.NoError(t, err)

	t.Log("Parsing templates...")
	err = chart.ParseTemplates()
	require.NoError(t, err)

	// Add debug logging
	t.Log("Found references:")
	for _, ref := range chart.References {
		t.Logf("Path: %s, Default: %s, Line: %d", ref.Path, ref.DefaultValue, ref.LineNumber)
	}

	t.Log("Loading values...")
	err = chart.LoadValues()
	require.NoError(t, err)

	t.Log("Updating values...")
	err = chart.UpdateValues()
	require.NoError(t, err)

	// Read the generated values.yaml
	data, err := os.ReadFile(filepath.Join(tmpDir, "values.yaml"))
	require.NoError(t, err)

	t.Logf("Generated values.yaml content:\n%s", string(data))

	var values map[string]interface{}
	err = yaml.Unmarshal(data, &values)
	require.NoError(t, err)

	// Debug output
	t.Logf("Final values: %+v", values)

	// Verify that the last default value was used
	assert.Equal(t, "stage.example.com", values["domain"])
}

func TestValuesHandling(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "helm-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create templates directory and test template
	templatesDir := filepath.Join(tmpDir, "templates")
	require.NoError(t, os.MkdirAll(templatesDir, 0755))

	templateContent := `
apiVersion: v1
kind: Service
metadata:
  name: {{ .Values.name }}
  labels:
    app: {{ .Values.labels.app | default "myapp" }}
spec:
  ports:
  - port: {{ .Values.service.port | default 80 }}
    targetPort: {{ .Values.service.targetPort }}`

	err = os.WriteFile(filepath.Join(templatesDir, "service.yaml"), []byte(templateContent), 0644)
	require.NoError(t, err)

	t.Run("load non-existent values file", func(t *testing.T) {
		chart, err := NewChart(tmpDir, nil)
		require.NoError(t, err)

		err = chart.LoadValues()
		assert.NoError(t, err)
		assert.Empty(t, chart.Values)
	})

	t.Run("load invalid values file", func(t *testing.T) {
		err := os.WriteFile(filepath.Join(tmpDir, "values.yaml"), []byte("invalid: yaml: :"), 0644)
		require.NoError(t, err)

		chart, err := NewChart(tmpDir, nil)
		require.NoError(t, err)

		err = chart.LoadValues()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "parsing values file")
	})

	t.Run("update values with defaults", func(t *testing.T) {
		// Create initial values
		initialValues := `
name: myservice
service:
  targetPort: 8080`

		err := os.WriteFile(filepath.Join(tmpDir, "values.yaml"), []byte(initialValues), 0644)
		require.NoError(t, err)

		chart, err := NewChart(tmpDir, nil)
		require.NoError(t, err)

		err = chart.LoadValues()
		require.NoError(t, err)

		err = chart.FindTemplates()
		require.NoError(t, err)

		err = chart.ParseTemplates()
		require.NoError(t, err)

		err = chart.UpdateValues()
		require.NoError(t, err)

		// Verify final values
		data, err := os.ReadFile(chart.ValuesFile)
		require.NoError(t, err)

		var values map[string]any
		err = yaml.Unmarshal(data, &values)
		require.NoError(t, err)

		// Check original values preserved
		assert.Equal(t, "myservice", values["name"])
		service := values["service"].(map[string]any)
		assert.Equal(t, "8080", service["targetPort"].(string))

		// Check new values added with defaults
		labels := values["labels"].(map[string]any)
		assert.Equal(t, "myapp", labels["app"].(string))
		assert.Equal(t, "80", service["port"].(string))
	})
}

func TestTemplateDiscovery(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "helm-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	templatesDir := filepath.Join(tmpDir, "templates")
	require.NoError(t, os.MkdirAll(templatesDir, 0755))

	files := map[string]string{
		"service.yaml":    "kind: Service",
		"deployment.yml":  "kind: Deployment",
		"helpers.tpl":     "{{- define \"helper\" -}}",
		"README.md":       "# Templates",
		"nested/pod.yaml": "kind: Pod",
	}

	for name, content := range files {
		path := filepath.Join(templatesDir, name)
		if filepath.Dir(path) != templatesDir {
			require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
		}
		require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	}

	chart, err := NewChart(tmpDir, nil)
	require.NoError(t, err)

	err = chart.FindTemplates()
	require.NoError(t, err)

	// Should find .yaml, .yml, and .tpl files, including in nested directories
	assert.Len(t, chart.Templates, 4)

	// Verify specific files found
	foundFiles := make(map[string]bool)
	for _, template := range chart.Templates {
		foundFiles[filepath.Base(template)] = true
	}

	assert.True(t, foundFiles["service.yaml"])
	assert.True(t, foundFiles["deployment.yml"])
	assert.True(t, foundFiles["helpers.tpl"])
	assert.True(t, foundFiles["pod.yaml"])
	assert.False(t, foundFiles["README.md"])
}

func TestTemplateParsingErrors(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "helm-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	templatesDir := filepath.Join(tmpDir, "templates")
	require.NoError(t, os.MkdirAll(templatesDir, 0755))

	t.Run("unreadable template", func(t *testing.T) {
		// Create a template file with no read permissions
		templatePath := filepath.Join(templatesDir, "unreadable.yaml")
		err := os.WriteFile(templatePath, []byte("content"), 0000)
		require.NoError(t, err)

		chart, err := NewChart(tmpDir, nil)
		require.NoError(t, err)

		err = chart.FindTemplates()
		require.NoError(t, err)

		err = chart.ParseTemplates()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "reading template")
	})
}

func TestValuesUpdateErrors(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "helm-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	templatesDir := filepath.Join(tmpDir, "templates")
	require.NoError(t, os.MkdirAll(templatesDir, 0755))

	// Create a test template
	templateContent := `
apiVersion: v1
kind: Service
metadata:
  name: {{ .Values.name }}`

	err = os.WriteFile(filepath.Join(templatesDir, "service.yaml"), []byte(templateContent), 0644)
	require.NoError(t, err)

	t.Run("unwritable values file directory", func(t *testing.T) {
		unwritableDir := filepath.Join(tmpDir, "unwritable")
		require.NoError(t, os.MkdirAll(unwritableDir, 0755))

		// Create templates directory in unwritable dir first
		unwritableTemplatesDir := filepath.Join(unwritableDir, "templates")
		require.NoError(t, os.MkdirAll(unwritableTemplatesDir, 0755))

		// Copy template file
		err = os.WriteFile(filepath.Join(unwritableTemplatesDir, "service.yaml"), []byte(templateContent), 0644)
		require.NoError(t, err)

		// Create values directory but make it unwritable
		valuesDir := filepath.Join(unwritableDir)
		require.NoError(t, os.MkdirAll(valuesDir, 0755))
		require.NoError(t, os.Chmod(valuesDir, 0555))
		defer func() {
			err := os.Chmod(valuesDir, 0755)
			assert.NoError(t, err)
		}()

		chart, err := NewChart(unwritableDir, nil)
		require.NoError(t, err)

		err = chart.FindTemplates()
		require.NoError(t, err)

		err = chart.ParseTemplates()
		require.NoError(t, err)

		chart.Changed = true // Force update attempt
		err = chart.UpdateValues()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "permission denied")
	})

	t.Run("unwritable values file", func(t *testing.T) {
		// Create values file with no write permissions
		valuesPath := filepath.Join(tmpDir, "values.yaml")
		err := os.WriteFile(valuesPath, []byte("name: test"), 0444)
		require.NoError(t, err)
		defer func() {
			err := os.Chmod(valuesPath, 0644)
			assert.NoError(t, err)
		}()

		chart, err := NewChart(tmpDir, nil)
		require.NoError(t, err)

		err = chart.FindTemplates()
		require.NoError(t, err)

		err = chart.ParseTemplates()
		require.NoError(t, err)

		chart.Changed = true // Force update attempt
		err = chart.UpdateValues()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "permission denied")
	})
}

func TestNestedValueConversion(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "helm-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	templatesDir := filepath.Join(tmpDir, "templates")
	require.NoError(t, os.MkdirAll(templatesDir, 0755))

	// Create a template with nested values
	templateContent := `
apiVersion: v1
kind: Service
metadata:
  name: {{ .Values.nested.name }}
  port: {{ .Values.nested.port | default 80 }}`

	err = os.WriteFile(filepath.Join(templatesDir, "service.yaml"), []byte(templateContent), 0644)
	require.NoError(t, err)

	// Create initial values with mixed types
	initialValues := `
nested:
  name: test
  number: 42
  float: 3.14
  existing: value`

	err = os.WriteFile(filepath.Join(tmpDir, "values.yaml"), []byte(initialValues), 0644)
	require.NoError(t, err)

	chart, err := NewChart(tmpDir, nil)
	require.NoError(t, err)

	err = chart.LoadValues()
	require.NoError(t, err)

	err = chart.FindTemplates()
	require.NoError(t, err)

	err = chart.ParseTemplates()
	require.NoError(t, err)

	err = chart.UpdateValues()
	require.NoError(t, err)

	// Verify final values
	data, err := os.ReadFile(chart.ValuesFile)
	require.NoError(t, err)

	var values map[string]any
	err = yaml.Unmarshal(data, &values)
	require.NoError(t, err)

	nested := values["nested"].(map[string]any)
	assert.Equal(t, "test", nested["name"])
	assert.Equal(t, "42", nested["number"])
	assert.Equal(t, "3.14", nested["float"])
	assert.Equal(t, "value", nested["existing"])
	assert.Equal(t, "80", nested["port"])
}
