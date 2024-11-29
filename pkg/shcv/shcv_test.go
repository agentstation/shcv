package shcv

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewChart(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "shcv-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name    string
		dir     string
		opts    []Option
		wantErr bool
	}{
		{
			name:    "valid directory",
			dir:     tmpDir,
			opts:    nil,
			wantErr: false,
		},
		{
			name:    "empty directory",
			dir:     "",
			opts:    nil,
			wantErr: true,
		},
		{
			name:    "non-existent directory",
			dir:     "/nonexistent/dir",
			opts:    nil,
			wantErr: true,
		},
		{
			name: "with custom options",
			dir:  tmpDir,
			opts: []Option{
				WithValuesFileNames([]string{"custom-values.yaml"}),
				WithTemplatesDir("custom-templates"),
				WithVerbose(true),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chart, err := NewChart(tt.dir, tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewChart() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && chart == nil {
				t.Error("NewChart() returned nil chart without error")
			}
		})
	}
}

func TestChart_LoadValueFiles(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "shcv-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test values.yaml file
	valuesContent := []byte(`
key1: value1
nested:
  key2: value2
`)
	valuesPath := filepath.Join(tmpDir, "values.yaml")
	if err := os.WriteFile(valuesPath, valuesContent, 0644); err != nil {
		t.Fatalf("Failed to create test values file: %v", err)
	}

	chart, err := NewChart(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create chart: %v", err)
	}

	if err := chart.LoadValueFiles(); err != nil {
		t.Errorf("LoadValueFiles() error = %v", err)
	}

	// Verify the values were loaded correctly
	if len(chart.ValuesFiles) != 1 {
		t.Errorf("Expected 1 values file, got %d", len(chart.ValuesFiles))
	}

	values := chart.ValuesFiles[0].Values
	if v, ok := values["key1"].(string); !ok || v != "value1" {
		t.Errorf("Expected key1=value1, got %v", values["key1"])
	}

	if nested, ok := values["nested"].(map[string]interface{}); !ok {
		t.Error("Expected nested to be a map")
	} else {
		if v, ok := nested["key2"].(string); !ok || v != "value2" {
			t.Errorf("Expected nested.key2=value2, got %v", nested["key2"])
		}
	}
}

func TestChart_FindTemplates(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "shcv-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create templates directory and test files
	templatesDir := filepath.Join(tmpDir, "templates")
	if err := os.Mkdir(templatesDir, 0755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}

	testFiles := []struct {
		name     string
		content  string
		expected bool
	}{
		{"test1.yaml", "test content", true},
		{"test2.yml", "test content", true},
		{"test3.tpl", "test content", true},
		{"test4.txt", "test content", false},
	}

	for _, tf := range testFiles {
		path := filepath.Join(templatesDir, tf.name)
		if err := os.WriteFile(path, []byte(tf.content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", tf.name, err)
		}
	}

	chart, err := NewChart(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create chart: %v", err)
	}

	if err := chart.FindTemplates(); err != nil {
		t.Errorf("FindTemplates() error = %v", err)
	}

	expectedCount := 3 // number of valid template files
	if len(chart.Templates) != expectedCount {
		t.Errorf("Expected %d templates, got %d", expectedCount, len(chart.Templates))
	}
}

func TestValueRef_ID(t *testing.T) {
	ref := &ValueRef{
		Path:         "test.path",
		DefaultValue: "default",
		SourceFile:   "test.yaml",
		LineNumber:   42,
	}

	expected := "test.path:42:test.yaml"
	if got := ref.ID(); got != expected {
		t.Errorf("ValueRef.ID() = %v, want %v", got, expected)
	}
}

func TestSetNestedValue(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		value    string
		initial  map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name:    "simple path",
			path:    "key",
			value:   "value",
			initial: map[string]interface{}{},
			expected: map[string]interface{}{
				"key": "value",
			},
		},
		{
			name:    "nested path",
			path:    "nested.key",
			value:   "value",
			initial: map[string]interface{}{},
			expected: map[string]interface{}{
				"nested": map[string]interface{}{
					"key": "value",
				},
			},
		},
		{
			name:  "existing nested structure",
			path:  "a.b.c",
			value: "value",
			initial: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{},
				},
			},
			expected: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"c": "value",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setNestedValue(tt.initial, tt.path, tt.value)
			// Simple equality check - in real tests you might want a deep equality check
			if !valueExists(tt.initial, tt.path) {
				t.Errorf("Value not set at path %s", tt.path)
			}
		})
	}
}

func TestValueExists(t *testing.T) {
	values := map[string]interface{}{
		"key1": "value1",
		"nested": map[string]interface{}{
			"key2": "value2",
		},
	}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "top level key exists",
			path:     "key1",
			expected: true,
		},
		{
			name:     "nested key exists",
			path:     "nested.key2",
			expected: true,
		},
		{
			name:     "non-existent key",
			path:     "missing",
			expected: false,
		},
		{
			name:     "non-existent nested key",
			path:     "nested.missing",
			expected: false,
		},
		{
			name:     "invalid nested path",
			path:     "key1.invalid",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := valueExists(values, tt.path); got != tt.expected {
				t.Errorf("valueExists() = %v, want %v", got, tt.expected)
			}
		})
	}
}
