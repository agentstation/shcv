package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/shcv/pkg/shcv"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func executeCommand(args ...string) (string, error) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = append([]string{"shcv"}, args...)

	var buf bytes.Buffer
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Reset command state
	RootCmd.ResetFlags()
	RootCmd.Flags().BoolP("verbose", "v", false, "verbose output showing all found references")

	err := RootCmd.Execute()

	w.Close()
	os.Stdout = old

	// Update to handle ReadFrom error
	_, readErr := buf.ReadFrom(r)
	if readErr != nil {
		return "", readErr
	}
	return buf.String(), err
}

func TestCLIFlags(t *testing.T) {
	// Test version flag first (simpler test)
	output, err := executeCommand("--version")
	assert.NoError(t, err)
	assert.Equal(t, shcv.Version+"\n", output)

	// Test verbose flag
	tmpDir, err := os.MkdirTemp("", "helm-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create templates directory with a simple template
	templatesDir := filepath.Join(tmpDir, "templates")
	err = os.MkdirAll(templatesDir, 0755)
	assert.NoError(t, err)

	templateContent := `apiVersion: v1
kind: Service
metadata:
  name: {{ .Values.name }}
spec:
  ports:
  - port: {{ .Values.port | default 80 }}`

	err = os.WriteFile(filepath.Join(templatesDir, "service.yaml"), []byte(templateContent), 0644)
	assert.NoError(t, err)

	// Test verbose output
	output, err = executeCommand("-v", tmpDir)
	assert.NoError(t, err)

	// Verify verbose output contains expected information
	assert.Contains(t, output, "Found 1 template files")
	assert.Contains(t, output, "Found 2 value references")
	assert.Contains(t, output, "name")
	assert.Contains(t, output, "port")
	assert.Contains(t, output, "default: 80")
	assert.Contains(t, output, "Successfully updated")

	// Test non-verbose output
	output, err = executeCommand(tmpDir)
	assert.NoError(t, err)
	assert.Empty(t, output)
}

func TestCLIErrors(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expectedError string
	}{
		{
			name:          "no arguments",
			args:          []string{},
			expectedError: "accepts 1 arg(s), received 0",
		},
		{
			name:          "too many arguments",
			args:          []string{"dir1", "dir2"},
			expectedError: "accepts 1 arg(s), received 2",
		},
		{
			name:          "non-existent directory",
			args:          []string{"/nonexistent"},
			expectedError: "error creating chart: invalid chart directory",
		},
		{
			name:          "no templates directory",
			args:          []string{os.TempDir()},
			expectedError: "error finding templates: templates directory not found",
		},
		{
			name:          "empty path",
			args:          []string{""},
			expectedError: "error creating chart: chart directory cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := executeCommand(tt.args...)
			if assert.Error(t, err) {
				assert.Contains(t, err.Error(), tt.expectedError)
			}
			assert.Empty(t, output)
		})
	}
}

func TestCLIValuesUpdate(t *testing.T) {
	// Create temporary directory for test chart
	tmpDir, err := os.MkdirTemp("", "helm-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create templates directory
	templatesDir := filepath.Join(tmpDir, "templates")
	err = os.MkdirAll(templatesDir, 0755)
	assert.NoError(t, err)

	// Create test template with new value
	templateContent := `apiVersion: v1
kind: Service
metadata:
  name: {{ .Values.name }}
  labels:
    app: {{ .Values.app | default "myapp" }}`

	err = os.WriteFile(filepath.Join(templatesDir, "service.yaml"), []byte(templateContent), 0644)
	assert.NoError(t, err)

	// Create initial values.yaml
	initialValues := `name: test-service`
	err = os.WriteFile(filepath.Join(tmpDir, "values.yaml"), []byte(initialValues), 0644)
	assert.NoError(t, err)

	// Process chart using CLI
	output, err := executeCommand(tmpDir)
	assert.NoError(t, err)
	assert.Empty(t, output)

	// Verify values.yaml was updated with new field
	data, err := os.ReadFile(filepath.Join(tmpDir, "values.yaml"))
	assert.NoError(t, err)

	var values map[string]any
	err = yaml.Unmarshal(data, &values)
	assert.NoError(t, err)

	// Check that existing value was preserved
	assert.Equal(t, "test-service", values["name"])
	// Check that new value was added with default
	assert.Equal(t, "myapp", values["app"])
}

func TestCLIOutputRedirectionErrors(t *testing.T) {
	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()

	r, w, _ := os.Pipe()
	os.Stdout = w

	// Start command in goroutine
	done := make(chan error)
	go func() {
		output, err := executeCommand("--version")
		if err != nil {
			done <- err
			return
		}
		done <- fmt.Errorf("expected error, got output: %s", output)
	}()

	// Close write end to force error
	w.Close()
	r.Close()

	// Wait for command to complete
	err := <-done
	assert.Error(t, err)
}

func TestCLIInvalidFlags(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expectedError string
	}{
		{
			name:          "unknown flag",
			args:          []string{"--unknown-flag", "."},
			expectedError: "unknown flag: --unknown-flag",
		},
		{
			name:          "invalid verbose flag value",
			args:          []string{"--verbose=invalid", "."},
			expectedError: "invalid argument",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := executeCommand(tt.args...)
			if assert.Error(t, err) {
				assert.Contains(t, err.Error(), tt.expectedError)
			}
			assert.Empty(t, output)
		})
	}
}

func TestCLIProcessChartEdgeCases(t *testing.T) {
	// Create temporary directory for test chart
	tmpDir, err := os.MkdirTemp("", "helm-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create templates directory
	templatesDir := filepath.Join(tmpDir, "templates")
	err = os.MkdirAll(templatesDir, 0755)
	assert.NoError(t, err)

	t.Run("unreadable template directory", func(t *testing.T) {
		// Make templates directory unreadable
		err := os.Chmod(templatesDir, 0000)
		assert.NoError(t, err)
		defer func() {
			err := os.Chmod(templatesDir, 0755)
			assert.NoError(t, err)
		}()

		output, err := executeCommand(tmpDir)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error finding templates")
		assert.Empty(t, output)
	})

	t.Run("invalid values.yaml", func(t *testing.T) {
		// Create invalid values.yaml
		err := os.WriteFile(filepath.Join(tmpDir, "values.yaml"), []byte(": invalid"), 0644)
		assert.NoError(t, err)

		output, err := executeCommand(tmpDir)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error loading values")
		assert.Empty(t, output)
	})

	t.Run("verbose output with no changes", func(t *testing.T) {
		// Create a new temporary directory for this test
		testDir, err := os.MkdirTemp("", "helm-test-verbose")
		assert.NoError(t, err)
		defer os.RemoveAll(testDir)

		// Create templates directory
		testTemplatesDir := filepath.Join(testDir, "templates")
		err = os.MkdirAll(testTemplatesDir, 0755)
		assert.NoError(t, err)

		// Create valid template with no values
		templateContent := `apiVersion: v1
kind: ConfigMap
metadata:
  name: static-config
`
		err = os.WriteFile(filepath.Join(testTemplatesDir, "configmap.yaml"), []byte(templateContent), 0644)
		assert.NoError(t, err)

		// Create empty but valid values.yaml
		err = os.WriteFile(filepath.Join(testDir, "values.yaml"), []byte("{}\n"), 0644)
		assert.NoError(t, err)

		output, err := executeCommand("-v", testDir)
		assert.NoError(t, err)
		assert.Contains(t, output, "Found 1 template files")
		assert.Contains(t, output, "Found 0 value references")
		assert.NotContains(t, output, "Successfully updated")
	})
}

func TestCLIVersionOutput(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"version flag", []string{"--version"}},
		{"version shorthand", []string{"-v", "--version"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := executeCommand(tt.args...)
			assert.NoError(t, err)
			assert.Equal(t, shcv.Version+"\n", output)
		})
	}
}
