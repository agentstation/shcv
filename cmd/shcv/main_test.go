package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCmd(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		setup       func() (string, func())
		wantErr     bool
		errContains string
	}{
		{
			name:        "no args",
			args:        []string{},
			wantErr:     true,
			errContains: "accepts 1 arg(s)",
		},
		{
			name:        "too many args",
			args:        []string{"dir1", "dir2"},
			wantErr:     true,
			errContains: "accepts 1 arg(s)",
		},
		{
			name: "valid chart directory",
			args: []string{"testdata/valid-chart"},
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
			name:        "invalid chart directory",
			args:        []string{"nonexistent"},
			wantErr:     true,
			errContains: "error creating chart",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var chartDir string
			if tt.setup != nil {
				var cleanup func()
				chartDir, cleanup = tt.setup()
				defer cleanup()
				tt.args[0] = chartDir
			}

			cmd := RootCmd
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				if chartDir != "" {
					// Verify values.yaml was updated with the new value
					content, err := os.ReadFile(filepath.Join(chartDir, "values.yaml"))
					require.NoError(t, err)
					assert.Contains(t, string(content), "newValue:")
				}
			}
		})
	}
}

func TestVerboseOutput(t *testing.T) {
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

	cmd := RootCmd
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"-v", chartDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String() + stderr.String()
	assert.Contains(t, output, "Found")
	assert.Contains(t, output, "template files")
	assert.Contains(t, output, "value references")
	assert.Contains(t, output, "deployment.yaml")
	assert.Contains(t, output, "default: defaultValue")
}

func TestVersionFlag(t *testing.T) {
	cmd := RootCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--version"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.True(t, len(strings.TrimSpace(output)) > 0)
}
