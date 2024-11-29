package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/shcv/pkg/shcv"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCommand(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantErr     bool
		errContains string
	}{
		{
			name:        "no args",
			args:        []string{},
			wantErr:     true,
			errContains: "accepts 1 arg",
		},
		{
			name:        "too many args",
			args:        []string{"dir1", "dir2"},
			wantErr:     true,
			errContains: "accepts 1 arg",
		},
		{
			name:        "invalid directory",
			args:        []string{"nonexistent"},
			wantErr:     true,
			errContains: "error creating chart",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create pipes to capture stdout and stderr
			oldStdout := os.Stdout
			oldStderr := os.Stderr
			r, w, _ := os.Pipe()
			os.Stdout = w
			os.Stderr = w
			defer func() {
				os.Stdout = oldStdout
				os.Stderr = oldStderr
			}()

			// Set command args
			cmd := RootCmd
			cmd.SetArgs(tt.args)

			// Execute command
			err := cmd.Execute()

			// Close pipe and restore stdout/stderr
			w.Close()
			var buf bytes.Buffer
			_, copyErr := io.Copy(&buf, r)
			require.NoError(t, copyErr)
			output := buf.String()

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, output, tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestVersionFlag(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	cmd := RootCmd
	cmd.SetArgs([]string{"--version"})
	err := cmd.Execute()
	assert.NoError(t, err)

	w.Close()
	var buf bytes.Buffer
	_, copyErr := io.Copy(&buf, r)
	require.NoError(t, copyErr)
	output := buf.String()

	assert.Equal(t, shcv.Version+"\n", output)
}

func TestHelpFlag(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	cmd := RootCmd
	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()
	assert.NoError(t, err)

	w.Close()
	var buf bytes.Buffer
	_, copyErr := io.Copy(&buf, r)
	require.NoError(t, copyErr)
	output := buf.String()

	assert.Contains(t, output, "Usage:")
	assert.Contains(t, output, "Flags:")
}

func TestProcessChart(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() (string, func())
		verbose     bool
		wantErr     bool
		errContains string
		validate    func(*testing.T, string, *bytes.Buffer)
	}{
		{
			name: "valid chart",
			setup: func() (string, func()) {
				dir := t.TempDir()
				chartDir := filepath.Join(dir, "valid-chart")
				require.NoError(t, os.MkdirAll(filepath.Join(chartDir, "templates"), 0755))
				require.NoError(t, os.WriteFile(
					filepath.Join(chartDir, "values.yaml"),
					[]byte("existing: value\n"),
					0644,
				))
				require.NoError(t, os.WriteFile(
					filepath.Join(chartDir, "templates/deployment.yaml"),
					[]byte("{{ .Values.newValue }}\n"),
					0644,
				))
				return chartDir, func() {}
			},
			validate: func(t *testing.T, chartDir string, _ *bytes.Buffer) {
				content, err := os.ReadFile(filepath.Join(chartDir, "values.yaml"))
				require.NoError(t, err)
				assert.Contains(t, string(content), "newValue:")
			},
		},
		{
			name: "valid chart with verbose",
			setup: func() (string, func()) {
				dir := t.TempDir()
				chartDir := filepath.Join(dir, "verbose-chart")
				require.NoError(t, os.MkdirAll(filepath.Join(chartDir, "templates"), 0755))
				require.NoError(t, os.WriteFile(
					filepath.Join(chartDir, "values.yaml"),
					[]byte("existing: value\n"),
					0644,
				))
				require.NoError(t, os.WriteFile(
					filepath.Join(chartDir, "templates/deployment.yaml"),
					[]byte("{{ .Values.newValue | default \"defaultValue\" }}\n"),
					0644,
				))
				return chartDir, func() {}
			},
			verbose: true,
			validate: func(t *testing.T, chartDir string, output *bytes.Buffer) {
				assert.Contains(t, output.String(), "Found")
				assert.Contains(t, output.String(), "template files")
				assert.Contains(t, output.String(), "value references")
				assert.Contains(t, output.String(), "deployment.yaml")
				assert.Contains(t, output.String(), "default: defaultValue")
			},
		},
		{
			name: "invalid chart directory",
			setup: func() (string, func()) {
				return "nonexistent", func() {}
			},
			wantErr:     true,
			errContains: "error creating chart",
		},
		{
			name: "invalid values file",
			setup: func() (string, func()) {
				dir := t.TempDir()
				chartDir := filepath.Join(dir, "invalid-values")
				require.NoError(t, os.MkdirAll(filepath.Join(chartDir, "templates"), 0755))
				require.NoError(t, os.WriteFile(
					filepath.Join(chartDir, "values.yaml"),
					[]byte("invalid: : yaml\n"),
					0644,
				))
				return chartDir, func() {}
			},
			wantErr:     true,
			errContains: "error loading values",
		},
		{
			name: "no templates directory",
			setup: func() (string, func()) {
				dir := t.TempDir()
				chartDir := filepath.Join(dir, "no-templates")
				require.NoError(t, os.MkdirAll(chartDir, 0755))
				require.NoError(t, os.WriteFile(
					filepath.Join(chartDir, "values.yaml"),
					[]byte("key: value\n"),
					0644,
				))
				return chartDir, func() {}
			},
			wantErr:     true,
			errContains: "error finding templates",
		},
		{
			name: "invalid template file",
			setup: func() (string, func()) {
				dir := t.TempDir()
				chartDir := filepath.Join(dir, "invalid-template")
				require.NoError(t, os.MkdirAll(filepath.Join(chartDir, "templates"), 0755))
				require.NoError(t, os.WriteFile(
					filepath.Join(chartDir, "values.yaml"),
					[]byte("key: value\n"),
					0644,
				))
				templatePath := filepath.Join(chartDir, "templates/deployment.yaml")
				require.NoError(t, os.WriteFile(templatePath, []byte("{{ .Values.key }}\n"), 0644))
				require.NoError(t, os.Chmod(templatePath, 0000))
				return chartDir, func() {
					require.NoError(t, os.Chmod(templatePath, 0644))
				}
			},
			wantErr:     true,
			errContains: "error parsing templates",
		},
		{
			name: "error updating values",
			setup: func() (string, func()) {
				dir := t.TempDir()
				chartDir := filepath.Join(dir, "update-error")
				require.NoError(t, os.MkdirAll(filepath.Join(chartDir, "templates"), 0755))
				valuesPath := filepath.Join(chartDir, "values.yaml")
				require.NoError(t, os.WriteFile(valuesPath, []byte("key: value\n"), 0644))
				require.NoError(t, os.WriteFile(
					filepath.Join(chartDir, "templates/deployment.yaml"),
					[]byte("{{ .Values.newValue }}\n"),
					0644,
				))
				// Create a symlink to a non-existent file
				require.NoError(t, os.Remove(valuesPath))
				require.NoError(t, os.Symlink("/nonexistent", valuesPath))
				return chartDir, func() {
					require.NoError(t, os.Remove(valuesPath))
				}
			},
			wantErr:     true,
			errContains: "error updating values: writing values file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chartDir, cleanup := tt.setup()
			defer cleanup()

			var output bytes.Buffer
			err := processChart(chartDir, tt.verbose, &output)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, chartDir, &output)
				}
			}
		})
	}
}

func TestMain(t *testing.T) {
	// Save original args and restore them after the test
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Save original stderr and restore it after the test
	oldStderr := os.Stderr
	defer func() { os.Stderr = oldStderr }()

	// Save original osExit and restore it after the test
	oldOsExit := osExit
	defer func() { osExit = oldOsExit }()

	tests := []struct {
		name        string
		args        []string
		setup       func() (string, func())
		wantErr     bool
		errContains string
	}{
		{
			name: "valid chart",
			setup: func() (string, func()) {
				dir := t.TempDir()
				chartDir := filepath.Join(dir, "valid-chart")
				require.NoError(t, os.MkdirAll(filepath.Join(chartDir, "templates"), 0755))
				require.NoError(t, os.WriteFile(
					filepath.Join(chartDir, "values.yaml"),
					[]byte("existing: value\n"),
					0644,
				))
				require.NoError(t, os.WriteFile(
					filepath.Join(chartDir, "templates/deployment.yaml"),
					[]byte("{{ .Values.newValue }}\n"),
					0644,
				))
				return chartDir, func() {}
			},
			wantErr: false,
		},
		{
			name:        "no args",
			args:        []string{"shcv"},
			wantErr:     true,
			errContains: "accepts 1 arg(s), received 0",
		},
		{
			name:        "too many args",
			args:        []string{"shcv", "arg1", "arg2"},
			wantErr:     true,
			errContains: "accepts 1 arg(s), received 2",
		},
		{
			name:        "nonexistent directory",
			args:        []string{"shcv", "nonexistent"},
			wantErr:     true,
			errContains: "error creating chart",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up test directory if needed
			var chartDir string
			var cleanup func()
			if tt.setup != nil {
				chartDir, cleanup = tt.setup()
				defer cleanup()
			}

			// Set up args
			if len(tt.args) > 0 {
				os.Args = tt.args
			} else {
				os.Args = []string{"shcv", chartDir}
			}

			// Capture stderr
			r, w, _ := os.Pipe()
			os.Stderr = w

			// Mock osExit
			var exitCode int
			osExit = func(code int) {
				exitCode = code
				panic(code)
			}

			// Run main and capture exit code
			func() {
				defer func() {
					if r := recover(); r != nil {
						if code, ok := r.(int); ok {
							exitCode = code
						}
					}
				}()
				main()
			}()

			// Close write end of pipe and read all output
			w.Close()
			var buf bytes.Buffer
			_, err := io.Copy(&buf, r)
			require.NoError(t, err)
			stderr := buf.String()

			// Reset RootCmd for next test
			RootCmd = &cobra.Command{
				Use:   "shcv [chart-directory]",
				Short: "Sync Helm Chart Values",
				Long: `shcv (Sync Helm Chart Values) is a tool that helps maintain Helm chart values
by automatically synchronizing values.yaml with the parameters used in your Helm templates.

It scans all template files for {{ .Values.* }} expressions and ensures they are properly
defined in your values file, including handling of default values and nested structures.

Example:
  shcv ./my-helm-chart`,
				Args: cobra.ExactArgs(1),
				RunE: func(cmd *cobra.Command, args []string) error {
					verbose, _ := cmd.Flags().GetBool("verbose")
					return processChart(args[0], verbose, cmd.OutOrStdout())
				},
				Version: shcv.Version,
			}
			RootCmd.Flags().BoolP("verbose", "v", false, "verbose output showing all found references")
			RootCmd.SetVersionTemplate(`{{.Version}}
`)

			if tt.wantErr {
				assert.Equal(t, 1, exitCode)
				if tt.errContains != "" {
					assert.Contains(t, stderr, tt.errContains)
				}
			} else {
				assert.Equal(t, 0, exitCode)
			}
		})
	}
}
