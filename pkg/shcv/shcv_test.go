package shcv

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	tests := []struct {
		name          string
		templatesDir  string
		setup         func(string, string) error
		cleanup       func(string, string) error
		wantTemplates []string
		wantErr       bool
	}{
		{
			name:         "find yaml templates",
			templatesDir: "templates",
			setup: func(dir, templatesDir string) error {
				templatesPath := filepath.Join(dir, templatesDir)
				if err := os.MkdirAll(templatesPath, 0755); err != nil {
					return err
				}
				files := map[string]string{
					"deployment.yaml": "",
					"service.yml":     "",
					"ingress.tpl":     "",
					"README.md":       "", // Should be ignored
				}
				for name, content := range files {
					if err := os.WriteFile(filepath.Join(templatesPath, name), []byte(content), 0644); err != nil {
						return err
					}
				}
				return nil
			},
			wantTemplates: []string{
				"deployment.yaml",
				"ingress.tpl",
				"service.yml",
			},
		},
		{
			name:         "empty templates directory",
			templatesDir: "templates",
			setup: func(dir, templatesDir string) error {
				return os.MkdirAll(filepath.Join(dir, templatesDir), 0755)
			},
			wantTemplates: nil,
		},
		{
			name:         "nonexistent templates directory",
			templatesDir: "nonexistent",
			wantErr:      true,
		},
		{
			name:         "nested templates",
			templatesDir: "templates",
			setup: func(dir, templatesDir string) error {
				templatesPath := filepath.Join(dir, templatesDir)
				nestedPath := filepath.Join(templatesPath, "nested")
				if err := os.MkdirAll(nestedPath, 0755); err != nil {
					return err
				}
				files := map[string]string{
					"deployment.yaml":       "",
					"nested/configmap.yaml": "",
				}
				for name, content := range files {
					if err := os.WriteFile(filepath.Join(templatesPath, name), []byte(content), 0644); err != nil {
						return err
					}
				}
				return nil
			},
			wantTemplates: []string{
				"deployment.yaml",
				filepath.Join("nested", "configmap.yaml"),
			},
		},
		{
			name:         "permission error",
			templatesDir: "templates",
			setup: func(dir, templatesDir string) error {
				templatesPath := filepath.Join(dir, templatesDir)
				if err := os.MkdirAll(templatesPath, 0755); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(templatesPath, "test.yaml"), []byte(""), 0644); err != nil {
					return err
				}
				return os.Chmod(templatesPath, 0000)
			},
			cleanup: func(dir, templatesDir string) error {
				return os.Chmod(filepath.Join(dir, templatesDir), 0755)
			},
			wantErr: true,
		},
		{
			name:         "mixed file types",
			templatesDir: "templates",
			setup: func(dir, templatesDir string) error {
				templatesPath := filepath.Join(dir, templatesDir)
				if err := os.MkdirAll(templatesPath, 0755); err != nil {
					return err
				}
				files := map[string]string{
					"deployment.yaml": "",
					"service.yml":     "",
					"ingress.tpl":     "",
					"script.sh":       "", // Should be ignored
					"notes.txt":       "", // Should be ignored
					".hidden.yaml":    "", // Hidden files are not ignored
				}
				for name, content := range files {
					if err := os.WriteFile(filepath.Join(templatesPath, name), []byte(content), 0644); err != nil {
						return err
					}
				}
				return nil
			},
			wantTemplates: []string{
				".hidden.yaml",
				"deployment.yaml",
				"ingress.tpl",
				"service.yml",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new temporary directory for each test case
			tempDir := t.TempDir()

			if tt.setup != nil {
				err := tt.setup(tempDir, tt.templatesDir)
				require.NoError(t, err)
			}

			// Ensure cleanup runs after the test
			if tt.cleanup != nil {
				defer func() {
					err := tt.cleanup(tempDir, tt.templatesDir)
					require.NoError(t, err)
				}()
			}

			chart := &Chart{
				Dir: tempDir,
				config: &config{
					TemplatesDir: tt.templatesDir,
				},
			}

			err := chart.FindTemplates()

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Get relative paths for comparison
				var relPaths []string
				for _, template := range chart.Templates {
					relPath, err := filepath.Rel(filepath.Join(tempDir, tt.templatesDir), template)
					require.NoError(t, err)
					relPaths = append(relPaths, relPath)
				}

				// Sort both slices for comparison
				sort.Strings(relPaths)
				sort.Strings(tt.wantTemplates)
				assert.Equal(t, tt.wantTemplates, relPaths)
			}
		})
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
		values   map[string]interface{}
		path     string
		value    string
		expected map[string]interface{}
	}{
		{
			name:   "simple value",
			values: make(map[string]interface{}),
			path:   "key",
			value:  "value",
			expected: map[string]interface{}{
				"key": "value",
			},
		},
		{
			name:   "nested value",
			values: make(map[string]interface{}),
			path:   "parent.child",
			value:  "value",
			expected: map[string]interface{}{
				"parent": map[string]interface{}{
					"child": "value",
				},
			},
		},
		{
			name: "update existing value",
			values: map[string]interface{}{
				"key": "old",
			},
			path:  "key",
			value: "new",
			expected: map[string]interface{}{
				"key": "new",
			},
		},
		{
			name: "update nested existing value",
			values: map[string]interface{}{
				"parent": map[string]interface{}{
					"child": "old",
				},
			},
			path:  "parent.child",
			value: "new",
			expected: map[string]interface{}{
				"parent": map[string]interface{}{
					"child": "new",
				},
			},
		},
		{
			name:   "deep nesting",
			values: make(map[string]interface{}),
			path:   "a.b.c.d",
			value:  "value",
			expected: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"c": map[string]interface{}{
							"d": "value",
						},
					},
				},
			},
		},
		{
			name: "convert existing value to map",
			values: map[string]interface{}{
				"parent": "old",
			},
			path:  "parent.child",
			value: "value",
			expected: map[string]interface{}{
				"parent": map[string]interface{}{
					"child": "value",
				},
			},
		},
		{
			name: "multiple paths",
			values: map[string]interface{}{
				"parent": map[string]interface{}{
					"child1": "value1",
				},
			},
			path:  "parent.child2",
			value: "value2",
			expected: map[string]interface{}{
				"parent": map[string]interface{}{
					"child1": "value1",
					"child2": "value2",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setNestedValue(tt.values, tt.path, tt.value)
			assert.Equal(t, tt.expected, tt.values)
		})
	}
}

func TestValueExists(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]interface{}
		path   string
		want   bool
	}{
		{
			name: "simple value exists",
			values: map[string]interface{}{
				"key": "value",
			},
			path: "key",
			want: true,
		},
		{
			name: "simple value does not exist",
			values: map[string]interface{}{
				"key": "value",
			},
			path: "nonexistent",
			want: false,
		},
		{
			name: "nested value exists",
			values: map[string]interface{}{
				"parent": map[string]interface{}{
					"child": "value",
				},
			},
			path: "parent.child",
			want: true,
		},
		{
			name: "nested value does not exist",
			values: map[string]interface{}{
				"parent": map[string]interface{}{
					"child": "value",
				},
			},
			path: "parent.nonexistent",
			want: false,
		},
		{
			name: "deep nested value exists",
			values: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"c": "value",
					},
				},
			},
			path: "a.b.c",
			want: true,
		},
		{
			name: "partial path exists",
			values: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{},
				},
			},
			path: "a.b.c",
			want: false,
		},
		{
			name: "path through non-map value",
			values: map[string]interface{}{
				"a": "value",
			},
			path: "a.b",
			want: false,
		},
		{
			name:   "empty values",
			values: map[string]interface{}{},
			path:   "key",
			want:   false,
		},
		{
			name: "empty path",
			values: map[string]interface{}{
				"": "value",
			},
			path: "",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := valueExists(tt.values, tt.path)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseTemplates(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		templates   []string
		verbose     bool
		setup       func(string) error
		wantErr     bool
		wantRefs    []ValueRef
		errContains string
	}{
		{
			name: "single template",
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "test.yaml"), []byte("{{ .Values.key }}\n"), 0644)
			},
			templates: []string{"test.yaml"},
			wantRefs: []ValueRef{
				{
					Path:       "key",
					SourceFile: "test.yaml",
					LineNumber: 1,
				},
			},
		},
		{
			name: "multiple templates",
			setup: func(dir string) error {
				if err := os.WriteFile(filepath.Join(dir, "test1.yaml"), []byte("{{ .Values.key1 }}\n"), 0644); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(dir, "test2.yaml"), []byte("{{ .Values.key2 }}\n"), 0644)
			},
			templates: []string{"test1.yaml", "test2.yaml"},
			wantRefs: []ValueRef{
				{
					Path:       "key1",
					SourceFile: "test1.yaml",
					LineNumber: 1,
				},
				{
					Path:       "key2",
					SourceFile: "test2.yaml",
					LineNumber: 1,
				},
			},
		},
		{
			name: "template with default values",
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "test.yaml"), []byte("{{ .Values.key | default \"value\" }}\n"), 0644)
			},
			templates: []string{"test.yaml"},
			wantRefs: []ValueRef{
				{
					Path:         "key",
					SourceFile:   "test.yaml",
					LineNumber:   1,
					DefaultValue: "value",
				},
			},
		},
		{
			name:        "nonexistent template",
			templates:   []string{"nonexistent.yaml"},
			wantErr:     true,
			errContains: "opening template",
		},
		{
			name: "permission error",
			setup: func(dir string) error {
				path := filepath.Join(dir, "noperm.yaml")
				if err := os.WriteFile(path, []byte("{{ .Values.key }}\n"), 0644); err != nil {
					return err
				}
				return os.Chmod(path, 0000)
			},
			templates:   []string{"noperm.yaml"},
			wantErr:     true,
			errContains: "opening template",
		},
		{
			name: "verbose output",
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "test.yaml"), []byte("{{ .Values.key }}\n"), 0644)
			},
			templates: []string{"test.yaml"},
			verbose:   true,
			wantRefs: []ValueRef{
				{
					Path:       "key",
					SourceFile: "test.yaml",
					LineNumber: 1,
				},
			},
		},
		{
			name: "invalid template content",
			setup: func(dir string) error {
				// Create a file with a line that's too long for the scanner
				var longLine strings.Builder
				for i := 0; i < bufio.MaxScanTokenSize+1; i++ {
					longLine.WriteByte('a')
				}
				return os.WriteFile(filepath.Join(dir, "test.yaml"), []byte(longLine.String()), 0644)
			},
			templates:   []string{"test.yaml"},
			wantErr:     true,
			errContains: "scanning template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test files
			if tt.setup != nil {
				err := tt.setup(tempDir)
				require.NoError(t, err)
			}

			// Update template paths to use temp directory
			templates := make([]string, len(tt.templates))
			for i, template := range tt.templates {
				templates[i] = filepath.Join(tempDir, template)
			}

			chart := &Chart{
				Templates: templates,
				config:    &config{Verbose: tt.verbose},
			}

			err := chart.ParseTemplates()

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.wantRefs), len(chart.References))
				for i, ref := range tt.wantRefs {
					assert.Equal(t, ref.Path, chart.References[i].Path)
					assert.Equal(t, ref.DefaultValue, chart.References[i].DefaultValue)
					assert.Equal(t, ref.LineNumber, chart.References[i].LineNumber)
					assert.Contains(t, chart.References[i].SourceFile, ref.SourceFile)
				}
			}
		})
	}
}

func TestProcessReferences(t *testing.T) {
	tests := []struct {
		name      string
		refs      []ValueRef
		values    []ValueFile
		wantPaths []string
	}{
		{
			name: "simple references",
			refs: []ValueRef{
				{Path: "simple", DefaultValue: ""},
				{Path: "withDefault", DefaultValue: "default"},
			},
			values: []ValueFile{
				{Path: "values.yaml", Values: make(map[string]interface{})},
			},
			wantPaths: []string{"simple", "withDefault"},
		},
		{
			name: "duplicate references with different defaults",
			refs: []ValueRef{
				{Path: "duplicate", DefaultValue: ""},
				{Path: "duplicate", DefaultValue: "first"},
				{Path: "duplicate", DefaultValue: "second"},
			},
			values: []ValueFile{
				{Path: "values.yaml", Values: make(map[string]interface{})},
			},
			wantPaths: []string{"duplicate"},
		},
		{
			name: "nested references",
			refs: []ValueRef{
				{Path: "parent.child", DefaultValue: "value"},
				{Path: "parent.sibling", DefaultValue: ""},
			},
			values: []ValueFile{
				{Path: "values.yaml", Values: make(map[string]interface{})},
			},
			wantPaths: []string{"parent.child", "parent.sibling"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chart := &Chart{
				References:  tt.refs,
				ValuesFiles: tt.values,
			}

			chart.ProcessReferences()

			// Verify all expected paths exist in values
			for _, file := range chart.ValuesFiles {
				assert.True(t, file.Changed)
				for _, path := range tt.wantPaths {
					assert.True(t, valueExists(file.Values, path))
				}
			}
		})
	}
}

func TestUpdateValueFiles(t *testing.T) {
	tempDir := t.TempDir()

	// Create a complex number that can't be marshaled to YAML
	unmarshalable := complex(1, 2)

	tests := []struct {
		name     string
		files    []ValueFile
		verbose  bool
		setup    func(string) error
		wantErr  bool
		validate func(*testing.T, string)
	}{
		{
			name: "update single file",
			files: []ValueFile{
				{
					Path: "values.yaml",
					Values: map[string]interface{}{
						"key": "value",
					},
					Changed: true,
				},
			},
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "values.yaml"), []byte("key: oldvalue\n"), 0644)
			},
			validate: func(t *testing.T, dir string) {
				content, err := os.ReadFile(filepath.Join(dir, "values.yaml"))
				require.NoError(t, err)
				assert.Contains(t, string(content), "key: value")
			},
		},
		{
			name: "no changes needed",
			files: []ValueFile{
				{
					Path:    "values.yaml",
					Values:  map[string]interface{}{},
					Changed: false,
				},
			},
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "values.yaml"), []byte("key: value\n"), 0644)
			},
			validate: func(t *testing.T, dir string) {
				content, err := os.ReadFile(filepath.Join(dir, "values.yaml"))
				require.NoError(t, err)
				assert.Equal(t, "key: value\n", string(content))
			},
		},
		{
			name: "invalid directory",
			files: []ValueFile{
				{
					Path:    "/nonexistent/values.yaml",
					Values:  map[string]interface{}{},
					Changed: true,
				},
			},
			wantErr: true,
		},
		{
			name: "verbose output",
			files: []ValueFile{
				{
					Path: "values.yaml",
					Values: map[string]interface{}{
						"key": "value",
					},
					Changed: true,
				},
			},
			verbose: true,
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "values.yaml"), []byte("key: oldvalue\n"), 0644)
			},
			validate: func(t *testing.T, dir string) {
				content, err := os.ReadFile(filepath.Join(dir, "values.yaml"))
				require.NoError(t, err)
				assert.Contains(t, string(content), "key: value")
			},
		},
		{
			name: "invalid yaml values",
			files: []ValueFile{
				{
					Path: "values.yaml",
					Values: map[string]interface{}{
						"key": unmarshalable, // complex numbers cannot be marshaled to YAML
					},
					Changed: true,
				},
			},
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "values.yaml"), []byte("key: oldvalue\n"), 0644)
			},
			wantErr: true,
		},
		{
			name: "permission error",
			files: []ValueFile{
				{
					Path: "noperm.yaml",
					Values: map[string]interface{}{
						"key": "value",
					},
					Changed: true,
				},
			},
			setup: func(dir string) error {
				path := filepath.Join(dir, "noperm.yaml")
				if err := os.WriteFile(path, []byte("key: oldvalue\n"), 0644); err != nil {
					return err
				}
				return os.Chmod(path, 0000)
			},
			wantErr: true,
		},
		{
			name: "multiple files",
			files: []ValueFile{
				{
					Path: "values1.yaml",
					Values: map[string]interface{}{
						"key1": "value1",
					},
					Changed: true,
				},
				{
					Path: "values2.yaml",
					Values: map[string]interface{}{
						"key2": "value2",
					},
					Changed: true,
				},
			},
			setup: func(dir string) error {
				if err := os.WriteFile(filepath.Join(dir, "values1.yaml"), []byte("key1: oldvalue1\n"), 0644); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(dir, "values2.yaml"), []byte("key2: oldvalue2\n"), 0644)
			},
			validate: func(t *testing.T, dir string) {
				content1, err := os.ReadFile(filepath.Join(dir, "values1.yaml"))
				require.NoError(t, err)
				assert.Contains(t, string(content1), "key1: value1")

				content2, err := os.ReadFile(filepath.Join(dir, "values2.yaml"))
				require.NoError(t, err)
				assert.Contains(t, string(content2), "key2: value2")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				err := tt.setup(tempDir)
				require.NoError(t, err)
			}

			// Update paths to use temp directory
			for i := range tt.files {
				if !strings.HasPrefix(tt.files[i].Path, "/") {
					tt.files[i].Path = filepath.Join(tempDir, filepath.Base(tt.files[i].Path))
				}
			}

			chart := &Chart{
				ValuesFiles: tt.files,
				config:      &config{Verbose: tt.verbose},
			}

			var err error
			func() {
				defer func() {
					if r := recover(); r != nil {
						err = fmt.Errorf("marshaling values: %v", r)
					}
				}()
				err = chart.UpdateValueFiles()
			}()

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, tempDir)
				}
			}
		})
	}
}

func TestLoadValueFiles(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name     string
		files    []ValueFile
		verbose  bool
		setup    func(string) error
		wantErr  bool
		validate func(*testing.T, []ValueFile)
	}{
		{
			name: "load existing file",
			files: []ValueFile{
				{Path: "values.yaml"},
			},
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "values.yaml"), []byte("key: value\n"), 0644)
			},
			validate: func(t *testing.T, files []ValueFile) {
				require.Len(t, files, 1)
				assert.Equal(t, "value", files[0].Values["key"])
			},
		},
		{
			name: "load multiple files",
			files: []ValueFile{
				{Path: "values1.yaml"},
				{Path: "values2.yaml"},
			},
			setup: func(dir string) error {
				if err := os.WriteFile(filepath.Join(dir, "values1.yaml"), []byte("key1: value1\n"), 0644); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(dir, "values2.yaml"), []byte("key2: value2\n"), 0644)
			},
			validate: func(t *testing.T, files []ValueFile) {
				require.Len(t, files, 2)
				assert.Equal(t, "value1", files[0].Values["key1"])
				assert.Equal(t, "value2", files[1].Values["key2"])
			},
		},
		{
			name: "nonexistent file",
			files: []ValueFile{
				{Path: "nonexistent.yaml"},
			},
			validate: func(t *testing.T, files []ValueFile) {
				require.Len(t, files, 1)
				assert.NotNil(t, files[0].Values)
				assert.Len(t, files[0].Values, 0)
			},
		},
		{
			name: "invalid yaml",
			files: []ValueFile{
				{Path: "invalid.yaml"},
			},
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "invalid.yaml"), []byte("invalid: : yaml\n"), 0644)
			},
			wantErr: true,
		},
		{
			name: "verbose output",
			files: []ValueFile{
				{Path: "values.yaml"},
				{Path: "empty.yaml"},
			},
			verbose: true,
			setup: func(dir string) error {
				if err := os.WriteFile(filepath.Join(dir, "values.yaml"), []byte("key: value\n"), 0644); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(dir, "empty.yaml"), []byte(""), 0644)
			},
			validate: func(t *testing.T, files []ValueFile) {
				require.Len(t, files, 2)
				assert.Equal(t, "value", files[0].Values["key"])
				assert.NotNil(t, files[1].Values)
				assert.Len(t, files[1].Values, 0)
			},
		},
		{
			name: "file with existing values map",
			files: []ValueFile{
				{
					Path: "values.yaml",
					Values: map[string]interface{}{
						"existing": "value",
					},
				},
			},
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "values.yaml"), []byte("new: value\n"), 0644)
			},
			validate: func(t *testing.T, files []ValueFile) {
				require.Len(t, files, 1)
				assert.Equal(t, "value", files[0].Values["existing"])
				assert.Equal(t, "value", files[0].Values["new"])
			},
		},
		{
			name: "file with permission error",
			files: []ValueFile{
				{Path: "noperm.yaml"},
			},
			setup: func(dir string) error {
				path := filepath.Join(dir, "noperm.yaml")
				if err := os.WriteFile(path, []byte("key: value\n"), 0644); err != nil {
					return err
				}
				return os.Chmod(path, 0000)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				err := tt.setup(tempDir)
				require.NoError(t, err)
			}

			// Update paths to use temp directory
			for i := range tt.files {
				tt.files[i].Path = filepath.Join(tempDir, filepath.Base(tt.files[i].Path))
			}

			chart := &Chart{
				ValuesFiles: tt.files,
				config:      &config{Verbose: tt.verbose},
			}

			err := chart.LoadValueFiles()

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, chart.ValuesFiles)
				}
			}
		})
	}
}

func TestChart_InjectDeploymentStrategy(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name     string
		template string
		setup    func(string) error
		validate func(*testing.T, *Chart, string)
	}{
		{
			name:     "deployment manifest",
			template: "deployment.yaml",
			setup: func(dir string) error {
				content := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
spec:
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
      - name: test
        image: test:latest`
				return os.WriteFile(filepath.Join(dir, "deployment.yaml"), []byte(content), 0644)
			},
			validate: func(t *testing.T, chart *Chart, dir string) {
				assert.True(t, chart.ValuesFiles[0].Changed)
				strategy, ok := chart.ValuesFiles[0].Values["deployment"].(map[string]interface{})
				assert.True(t, ok)
				assert.NotNil(t, strategy["strategy"])
				strategyConfig := strategy["strategy"].(map[string]interface{})
				assert.Equal(t, "RollingUpdate", strategyConfig["type"])
				rollingUpdate := strategyConfig["rollingUpdate"].(map[string]interface{})
				assert.Equal(t, 1, rollingUpdate["maxSurge"])
				assert.Equal(t, 0, rollingUpdate["maxUnavailable"])
			},
		},
		{
			name:     "non-deployment manifest",
			template: "service.yaml",
			setup: func(dir string) error {
				content := `apiVersion: v1
kind: Service
metadata:
  name: test-service
spec:
  ports:
  - port: 80
    targetPort: 8080`
				return os.WriteFile(filepath.Join(dir, "service.yaml"), []byte(content), 0644)
			},
			validate: func(t *testing.T, chart *Chart, dir string) {
				assert.False(t, chart.ValuesFiles[0].Changed)
				_, ok := chart.ValuesFiles[0].Values["deployment"]
				assert.False(t, ok)
			},
		},
		{
			name:     "invalid yaml",
			template: "invalid.yaml",
			setup: func(dir string) error {
				content := `invalid: yaml: :`
				return os.WriteFile(filepath.Join(dir, "invalid.yaml"), []byte(content), 0644)
			},
			validate: func(t *testing.T, chart *Chart, dir string) {
				assert.False(t, chart.ValuesFiles[0].Changed)
			},
		},
		{
			name:     "existing strategy",
			template: "deployment.yaml",
			setup: func(dir string) error {
				content := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment`
				return os.WriteFile(filepath.Join(dir, "deployment.yaml"), []byte(content), 0644)
			},
			validate: func(t *testing.T, chart *Chart, dir string) {
				// Pre-populate values with existing strategy
				chart.ValuesFiles[0].Values = map[string]interface{}{
					"deployment": map[string]interface{}{
						"strategy": map[string]interface{}{
							"type": "Recreate",
						},
					},
				}
				chart.ValuesFiles[0].Changed = false // Reset the changed flag
				err := chart.injectDeploymentStrategy(filepath.Join(dir, "deployment.yaml"))
				assert.NoError(t, err)
				assert.False(t, chart.ValuesFiles[0].Changed)
				strategy := chart.ValuesFiles[0].Values["deployment"].(map[string]interface{})["strategy"].(map[string]interface{})
				assert.Equal(t, "Recreate", strategy["type"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				err := tt.setup(tempDir)
				require.NoError(t, err)
			}

			// Create chart with proper config initialization
			chart, err := NewChart(tempDir, WithVerbose(true))
			require.NoError(t, err)

			chart.ValuesFiles = []ValueFile{
				{
					Path:    filepath.Join(tempDir, "values.yaml"),
					Values:  make(map[string]interface{}),
					Changed: false,
				},
			}

			if tt.name == "existing strategy" {
				// Pre-populate values for the existing strategy test
				chart.ValuesFiles[0].Values = map[string]interface{}{
					"deployment": map[string]interface{}{
						"strategy": map[string]interface{}{
							"type": "Recreate",
						},
					},
				}
			}

			err = chart.injectDeploymentStrategy(filepath.Join(tempDir, tt.template))
			assert.NoError(t, err)

			if tt.validate != nil {
				tt.validate(t, chart, tempDir)
			}
		})
	}
}

func TestUpdateDeploymentTemplate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "basic deployment",
			input: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
      - name: test
        image: test:latest`,
			expected: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  strategy:
    type: {{ .Values.deployment.strategy.type }}
    rollingUpdate:
      maxSurge: {{ .Values.deployment.strategy.rollingUpdate.maxSurge }}
      maxUnavailable: {{ .Values.deployment.strategy.rollingUpdate.maxUnavailable }}
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
      - name: test
        image: test:latest`,
		},
		{
			name: "existing strategy",
			input: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: test`,
			expected: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: test`,
		},
		{
			name: "no spec section",
			input: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test`,
			expected: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test`,
		},
		{
			name: "complex deployment with includes",
			input: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "gateway.fullname" . }}-deployment
  labels:
    app.kubernetes.io/component: server
    app.kubernetes.io/part-of: gateway
  {{- include "gateway.labels" . | nindent 4 }}
spec:
  replicas: {{ .Values.deployment.replicas }}
  selector:
    matchLabels:
      app.kubernetes.io/component: server
      {{- include "gateway.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      labels:
        app.kubernetes.io/component: server
        {{- include "gateway.selectorLabels" . | nindent 8 }}
    spec:
      containers:
      - name: gateway
        image: {{ .Values.image }}`,
			expected: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "gateway.fullname" . }}-deployment
  labels:
    app.kubernetes.io/component: server
    app.kubernetes.io/part-of: gateway
  {{- include "gateway.labels" . | nindent 4 }}
spec:
  strategy:
    type: {{ .Values.deployment.strategy.type }}
    rollingUpdate:
      maxSurge: {{ .Values.deployment.strategy.rollingUpdate.maxSurge }}
      maxUnavailable: {{ .Values.deployment.strategy.rollingUpdate.maxUnavailable }}
  replicas: {{ .Values.deployment.replicas }}
  selector:
    matchLabels:
      app.kubernetes.io/component: server
      {{- include "gateway.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      labels:
        app.kubernetes.io/component: server
        {{- include "gateway.selectorLabels" . | nindent 8 }}
    spec:
      containers:
      - name: gateway
        image: {{ .Values.image }}`,
		},
		{
			name: "deployment with existing strategy",
			input: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: test`,
			expected: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: test`,
		},
		{
			name: "deployment with custom indentation",
			input: `apiVersion: apps/v1
kind: Deployment
metadata:
    name: test
spec:
    replicas: 3
    selector:
        matchLabels:
            app: test`,
			expected: `apiVersion: apps/v1
kind: Deployment
metadata:
    name: test
spec:
    strategy:
        type: {{ .Values.deployment.strategy.type }}
        rollingUpdate:
            maxSurge: {{ .Values.deployment.strategy.rollingUpdate.maxSurge }}
            maxUnavailable: {{ .Values.deployment.strategy.rollingUpdate.maxUnavailable }}
    replicas: 3
    selector:
        matchLabels:
            app: test`,
		},
		{
			name: "gateway deployment with complex structure",
			input: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "gateway.fullname" . }}-deployment
  labels:
    app.kubernetes.io/component: server
    app.kubernetes.io/part-of: gateway
  {{- include "gateway.labels" . | nindent 4 }}
  annotations:
    argocd.argoproj.io/sync-wave: "2"
spec:
  replicas: {{ .Values.deployment.replicas }}
  selector:
    matchLabels:
      app.kubernetes.io/component: server
      app.kubernetes.io/instance: gateway
      app.kubernetes.io/managed-by: kustomize
      app.kubernetes.io/name: gateway
      app.kubernetes.io/part-of: gateway
    {{- include "gateway.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      labels:
        app.kubernetes.io/component: server
        app.kubernetes.io/instance: gateway
        app.kubernetes.io/managed-by: kustomize
        app.kubernetes.io/name: gateway
        app.kubernetes.io/part-of: gateway
      {{- include "gateway.selectorLabels" . | nindent 8 }}
    spec:
      containers:
      - env:
        - name: ENV
          value: {{ quote .Values.deployment.gateway.env.env }}
        - name: LOG_LEVEL
          value: {{ quote .Values.deployment.gateway.env.logLevel }}
        - name: PORT
          value: {{ quote .Values.deployment.gateway.env.port }}
        - name: HOST
          value: {{ quote .Values.deployment.gateway.env.host }}
        - name: STARTUP_TIMEOUT
          value: {{ quote .Values.deployment.gateway.env.startupTimeout }}
        - name: SHUTDOWN_TIMEOUT
          value: {{ quote .Values.deployment.gateway.env.shutdownTimeout }}
        - name: STARTUP_CONN_RETRIES
          value: {{ quote .Values.deployment.gateway.env.startupConnRetries }}
        - name: STARTUP_CONN_TIMEOUT
          value: {{ quote .Values.deployment.gateway.env.startupConnTimeout }}
        - name: CORS_ALLOWED_ORIGINS
          value: {{ quote .Values.deployment.gateway.env.corsAllowedOrigins }}
        - name: CORS_ALLOWED_METHODS
          value: {{ quote .Values.deployment.gateway.env.corsAllowedMethods }}
        - name: CORS_ALLOWED_HEADERS
          value: {{ quote .Values.deployment.gateway.env.corsAllowedHeaders }}
        - name: CORS_ALLOW_CREDENTIALS
          value: {{ quote .Values.deployment.gateway.env.corsAllowCredentials }}
        - name: CORS_MAX_AGE
          value: {{ quote .Values.deployment.gateway.env.corsMaxAge }}
        - name: SBO_ADDR
          value: {{ quote .Values.deployment.gateway.env.sboAddr }}
        - name: POSTGRES_HOST
          valueFrom:
            secretKeyRef:
              key: {{ .Values.deployment.gateway.secrets.postgresConn.connectionNameKey | default "POSTGRES_CONNECTION_NAME" }}
              name: {{ .Values.deployment.gateway.secrets.postgresConn.name | default "postgres-conn-workload" }}
        - name: POSTGRES_PRIVATE_IP
          valueFrom:
            secretKeyRef:
              key: {{ .Values.deployment.gateway.secrets.postgresConn.privateIpKey | default "POSTGRES_PRIVATE_IP" }}
              name: {{ .Values.deployment.gateway.secrets.postgresConn.name | default "postgres-conn-workload" }}
        - name: POSTGRES_USER
          valueFrom:
            secretKeyRef:
              key: {{ .Values.deployment.gateway.secrets.postgresCreds.userKey | default "POSTGRES_AGENTSTATION_USER" }}
              name: {{ .Values.deployment.gateway.secrets.postgresCreds.name | default "postgres-creds-workload" }}
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              key: {{ .Values.deployment.gateway.secrets.postgresCreds.passwordKey | default "POSTGRES_AGENTSTATION_PASSWORD" }}
              name: {{ .Values.deployment.gateway.secrets.postgresCreds.name | default "postgres-creds-workload" }}
        - name: POSTGRES_DB
          value: {{ quote .Values.deployment.gateway.env.postgresDb }}
        - name: POSTGRES_SEARCH_PATH
          value: {{ quote .Values.deployment.gateway.env.postgresSearchPath }}
        - name: POSTGRES_SSL_MODE
          value: {{ quote .Values.deployment.gateway.env.postgresSslMode }}
        - name: POSTGRES_SSL_CERT
          valueFrom:
            secretKeyRef:
              key: {{ .Values.deployment.gateway.secrets.postgresConn.sslCertKey | default "POSTGRES_SERVER_CA_CERT" }}
              name: {{ .Values.deployment.gateway.secrets.postgresConn.name | default "postgres-conn-workload" }}
        - name: REDIS_ADDR
          value: {{ quote .Values.deployment.gateway.env.redisAddr }}
        - name: GA_MEASUREMENT_ID
          value: {{ quote .Values.deployment.gateway.env.gaMeasurementId }}
        - name: GA_API_SECRET
          value: {{ quote .Values.deployment.gateway.env.gaApiSecret }}
        - name: KUBERNETES_CLUSTER_DOMAIN
          value: {{ quote .Values.kubernetesClusterDomain }}
        image: {{ .Values.deployment.gateway.image.repository }}:{{ .Values.deployment.gateway.image.tag | default .Chart.AppVersion }}
        imagePullPolicy: {{ .Values.deployment.gateway.imagePullPolicy }}
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 15
          periodSeconds: 20
        name: gateway
        ports:
        - containerPort: 8080
          name: http
          protocol: TCP
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 5
        resources: {{- toYaml .Values.deployment.gateway.resources | nindent 10 }}
      serviceAccountName: {{ include "gateway.fullname" . }}-sa`,
			expected: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "gateway.fullname" . }}-deployment
  labels:
    app.kubernetes.io/component: server
    app.kubernetes.io/part-of: gateway
  {{- include "gateway.labels" . | nindent 4 }}
  annotations:
    argocd.argoproj.io/sync-wave: "2"
spec:
  strategy:
    type: {{ .Values.deployment.strategy.type }}
    rollingUpdate:
      maxSurge: {{ .Values.deployment.strategy.rollingUpdate.maxSurge }}
      maxUnavailable: {{ .Values.deployment.strategy.rollingUpdate.maxUnavailable }}
  replicas: {{ .Values.deployment.replicas }}
  selector:
    matchLabels:
      app.kubernetes.io/component: server
      app.kubernetes.io/instance: gateway
      app.kubernetes.io/managed-by: kustomize
      app.kubernetes.io/name: gateway
      app.kubernetes.io/part-of: gateway
    {{- include "gateway.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      labels:
        app.kubernetes.io/component: server
        app.kubernetes.io/instance: gateway
        app.kubernetes.io/managed-by: kustomize
        app.kubernetes.io/name: gateway
        app.kubernetes.io/part-of: gateway
      {{- include "gateway.selectorLabels" . | nindent 8 }}
    spec:
      containers:
      - env:
        - name: ENV
          value: {{ quote .Values.deployment.gateway.env.env }}
        - name: LOG_LEVEL
          value: {{ quote .Values.deployment.gateway.env.logLevel }}
        - name: PORT
          value: {{ quote .Values.deployment.gateway.env.port }}
        - name: HOST
          value: {{ quote .Values.deployment.gateway.env.host }}
        - name: STARTUP_TIMEOUT
          value: {{ quote .Values.deployment.gateway.env.startupTimeout }}
        - name: SHUTDOWN_TIMEOUT
          value: {{ quote .Values.deployment.gateway.env.shutdownTimeout }}
        - name: STARTUP_CONN_RETRIES
          value: {{ quote .Values.deployment.gateway.env.startupConnRetries }}
        - name: STARTUP_CONN_TIMEOUT
          value: {{ quote .Values.deployment.gateway.env.startupConnTimeout }}
        - name: CORS_ALLOWED_ORIGINS
          value: {{ quote .Values.deployment.gateway.env.corsAllowedOrigins }}
        - name: CORS_ALLOWED_METHODS
          value: {{ quote .Values.deployment.gateway.env.corsAllowedMethods }}
        - name: CORS_ALLOWED_HEADERS
          value: {{ quote .Values.deployment.gateway.env.corsAllowedHeaders }}
        - name: CORS_ALLOW_CREDENTIALS
          value: {{ quote .Values.deployment.gateway.env.corsAllowCredentials }}
        - name: CORS_MAX_AGE
          value: {{ quote .Values.deployment.gateway.env.corsMaxAge }}
        - name: SBO_ADDR
          value: {{ quote .Values.deployment.gateway.env.sboAddr }}
        - name: POSTGRES_HOST
          valueFrom:
            secretKeyRef:
              key: {{ .Values.deployment.gateway.secrets.postgresConn.connectionNameKey | default "POSTGRES_CONNECTION_NAME" }}
              name: {{ .Values.deployment.gateway.secrets.postgresConn.name | default "postgres-conn-workload" }}
        - name: POSTGRES_PRIVATE_IP
          valueFrom:
            secretKeyRef:
              key: {{ .Values.deployment.gateway.secrets.postgresConn.privateIpKey | default "POSTGRES_PRIVATE_IP" }}
              name: {{ .Values.deployment.gateway.secrets.postgresConn.name | default "postgres-conn-workload" }}
        - name: POSTGRES_USER
          valueFrom:
            secretKeyRef:
              key: {{ .Values.deployment.gateway.secrets.postgresCreds.userKey | default "POSTGRES_AGENTSTATION_USER" }}
              name: {{ .Values.deployment.gateway.secrets.postgresCreds.name | default "postgres-creds-workload" }}
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              key: {{ .Values.deployment.gateway.secrets.postgresCreds.passwordKey | default "POSTGRES_AGENTSTATION_PASSWORD" }}
              name: {{ .Values.deployment.gateway.secrets.postgresCreds.name | default "postgres-creds-workload" }}
        - name: POSTGRES_DB
          value: {{ quote .Values.deployment.gateway.env.postgresDb }}
        - name: POSTGRES_SEARCH_PATH
          value: {{ quote .Values.deployment.gateway.env.postgresSearchPath }}
        - name: POSTGRES_SSL_MODE
          value: {{ quote .Values.deployment.gateway.env.postgresSslMode }}
        - name: POSTGRES_SSL_CERT
          valueFrom:
            secretKeyRef:
              key: {{ .Values.deployment.gateway.secrets.postgresConn.sslCertKey | default "POSTGRES_SERVER_CA_CERT" }}
              name: {{ .Values.deployment.gateway.secrets.postgresConn.name | default "postgres-conn-workload" }}
        - name: REDIS_ADDR
          value: {{ quote .Values.deployment.gateway.env.redisAddr }}
        - name: GA_MEASUREMENT_ID
          value: {{ quote .Values.deployment.gateway.env.gaMeasurementId }}
        - name: GA_API_SECRET
          value: {{ quote .Values.deployment.gateway.env.gaApiSecret }}
        - name: KUBERNETES_CLUSTER_DOMAIN
          value: {{ quote .Values.kubernetesClusterDomain }}
        image: {{ .Values.deployment.gateway.image.repository }}:{{ .Values.deployment.gateway.image.tag | default .Chart.AppVersion }}
        imagePullPolicy: {{ .Values.deployment.gateway.imagePullPolicy }}
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 15
          periodSeconds: 20
        name: gateway
        ports:
        - containerPort: 8080
          name: http
          protocol: TCP
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 5
        resources: {{- toYaml .Values.deployment.gateway.resources | nindent 10 }}
      serviceAccountName: {{ include "gateway.fullname" . }}-sa`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := string(updateDeploymentTemplate([]byte(tt.input)))
			if result != tt.expected {
				t.Errorf("Expected:\n%s\n\nGot:\n%s", tt.expected, result)
			}
		})
	}
}

func TestGatewayDeploymentTemplate(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create test files
	deploymentPath := filepath.Join(tempDir, "templates", "deployment.yaml")
	valuesPath := filepath.Join(tempDir, "values.yaml")

	// Create directories
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "templates"), 0755))

	// Write the deployment template
	deploymentContent := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "gateway.fullname" . }}-deployment
  labels:
    app.kubernetes.io/component: server
    app.kubernetes.io/part-of: gateway
  {{- include "gateway.labels" . | nindent 4 }}
  annotations:
    argocd.argoproj.io/sync-wave: "2"
spec:
  replicas: {{ .Values.deployment.replicas }}
  selector:
    matchLabels:
      app.kubernetes.io/component: server
      app.kubernetes.io/instance: gateway
      app.kubernetes.io/managed-by: kustomize
      app.kubernetes.io/name: gateway
      app.kubernetes.io/part-of: gateway
    {{- include "gateway.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      labels:
        app.kubernetes.io/component: server
        app.kubernetes.io/instance: gateway
        app.kubernetes.io/managed-by: kustomize
        app.kubernetes.io/name: gateway
        app.kubernetes.io/part-of: gateway
      {{- include "gateway.selectorLabels" . | nindent 8 }}
    spec:
      containers:
      - env:
        - name: ENV
          value: {{ quote .Values.deployment.gateway.env.env }}`

	require.NoError(t, os.WriteFile(deploymentPath, []byte(deploymentContent), 0644))

	// Write values.yaml
	valuesContent := `deployment:
  replicas: 1
  gateway:
    env:
      env: "production"`

	require.NoError(t, os.WriteFile(valuesPath, []byte(valuesContent), 0644))

	// Create chart with verbose mode
	chart, err := NewChart(tempDir, WithVerbose(true))
	require.NoError(t, err)

	// Load values
	require.NoError(t, chart.LoadValueFiles())

	// Debug: Print initial values
	t.Logf("Initial values: %+v", chart.ValuesFiles[0].Values)

	// Set up templates manually
	chart.Templates = []string{deploymentPath}

	// Process references
	chart.ProcessReferences()

	// Debug: Print values after processing
	t.Logf("Values after processing: %+v", chart.ValuesFiles[0].Values)

	// Debug: Print deployment section
	if deployment, ok := chart.ValuesFiles[0].Values["deployment"].(map[string]interface{}); ok {
		t.Logf("Deployment section: %+v", deployment)
		if strategy, ok := deployment["strategy"].(map[string]interface{}); ok {
			t.Logf("Strategy section: %+v", strategy)
		} else {
			t.Log("No strategy section found")
		}
	} else {
		t.Log("No deployment section found")
	}

	// Verify the deployment strategy was added to values.yaml
	deployment, ok := chart.ValuesFiles[0].Values["deployment"].(map[string]interface{})
	require.True(t, ok, "deployment section should exist in values")

	strategy, ok := deployment["strategy"].(map[string]interface{})
	require.True(t, ok, "strategy section should exist in deployment")

	assert.Equal(t, "RollingUpdate", strategy["type"], "strategy type should be RollingUpdate")

	rollingUpdate, ok := strategy["rollingUpdate"].(map[string]interface{})
	require.True(t, ok, "rollingUpdate section should exist in strategy")
	assert.Equal(t, 1, rollingUpdate["maxSurge"], "maxSurge should be 1")
	assert.Equal(t, 0, rollingUpdate["maxUnavailable"], "maxUnavailable should be 0")

	// Read the updated deployment file
	updatedContent, err := os.ReadFile(deploymentPath)
	require.NoError(t, err)

	// Debug: Print updated content
	t.Logf("Updated deployment content:\n%s", string(updatedContent))

	// Verify the strategy section was added to the deployment template
	assert.Contains(t, string(updatedContent), "strategy:", "deployment should contain strategy section")
	assert.Contains(t, string(updatedContent), "type: {{ .Values.deployment.strategy.type }}", "deployment should contain strategy type")
	assert.Contains(t, string(updatedContent), "maxSurge: {{ .Values.deployment.strategy.rollingUpdate.maxSurge }}", "deployment should contain maxSurge")
	assert.Contains(t, string(updatedContent), "maxUnavailable: {{ .Values.deployment.strategy.rollingUpdate.maxUnavailable }}", "deployment should contain maxUnavailable")
}
