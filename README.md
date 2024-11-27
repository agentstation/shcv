# shcv (Sync Helm Chart Values)

[![Go Report Card](https://goreportcard.com/badge/github.com/agentstation/shcv)](https://goreportcard.com/report/github.com/agentstation/shcv)
[![GoDoc](https://godoc.org/github.com/agentstation/shcv?status.svg)](https://godoc.org/github.com/agentstation/shcv)
[![GitHub Workflow Status](https://img.shields.io/github/actions/workflow/status/agentstation/uuidkey/ci.yaml?style=flat-square)](https://github.com/agentstation/uuidkey/actions)
[![codecov](https://codecov.io/gh/agentstation/shcv/graph/badge.svg?token=7A0O794SOM)](https://codecov.io/gh/agentstation/shcv)
[![License](https://img.shields.io/github/license/agentstation/shcv.svg)](LICENSE)
[![Version](https://img.shields.io/badge/version-1.0.0-blue.svg)](https://github.com/agentstation/shcv/releases)
[![Go Version](https://img.shields.io/badge/go-%3E%3D%201.21-blue)](go.mod)

`shcv` is a command-line tool and Go package that helps maintain Helm chart values by automatically synchronizing `values.yaml` with the parameters used in your Helm templates. It scans all template files for `{{ .Values.* }}` expressions and ensures they are properly defined in your values file.

## Requirements

- Go 1.21 or later
- A valid Helm chart directory structure
- Read/write permissions for the chart directory

## Installation

```bash
go install github.com/agentstation/shcv@latest
```

## CLI Usage

```bash
# Process chart in current directory
shcv .

# Process chart with verbose output
shcv -v ./my-helm-chart

# Show help
shcv --help

# Show version
shcv --version
```

### Verbose Output Format

When using the `-v` or `--verbose` flag, the output includes:
- Number of template files found
- Number of value references discovered
- For each reference:
  - Full path (e.g., `gateway.domain`)
  - Source file and line number
  - Default value if specified

Example verbose output:
```
Found 2 template files
Found 5 value references
- gateway.domain (from ingress.yaml:12)
  default: api.example.com
- service.port (from ingress.yaml:18)
  default: 80
```

## Package Usage

The core functionality is available as a Go package that you can use in your own programs:

```go
import "github.com/agentstation/shcv/pkg/shcv"

// Use default options
chart, err := shcv.NewChart("./my-chart", nil)
if err != nil {
    log.Fatal(err)
}

// Load and process the chart
if err := chart.LoadValues(); err != nil {
    log.Fatal(err)
}
if err := chart.FindTemplates(); err != nil {
    log.Fatal(err)
}
if err := chart.ParseTemplates(); err != nil {
    log.Fatal(err)
}
if err := chart.UpdateValues(); err != nil {
    log.Fatal(err)
}

// Or customize the behavior
opts := &shcv.Options{
    ValuesFileName: "custom-values.yaml",
    TemplatesDir:   "custom-templates",
    DefaultValues: map[string]string{
        "domain":   "custom.example.com",
        "port":     "8080",
        "replicas": "3",
    },
}
chart, err := shcv.NewChart("./my-chart", opts)
```

## Example

Given a template file `templates/ingress.yaml`:
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ingress
  namespace: gateway
  annotations:
    replicas: {{ .Values.deployment.replicas | default 3 }}
    enabled: {{ .Values.features.monitoring.enabled | default "true" }}
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
```

Running `shcv` will create/update `values.yaml` with:
```yaml
deployment:
  replicas: "3"
features:
  monitoring:
    enabled: "true"
gateway:
  domain: "api.example.com"
path: "/"
service:
  port: "80"
```

## Features

- Automatically detects all Helm value references in template files
- Supports nested value structures (e.g., `{{ .Values.gateway.domain }}`)
- Handles default values in templates (e.g., `{{ .Values.domain | default "api.example.com" }}`)
- Supports double-quoted, single-quoted, and numeric default values
- Creates missing values in `values.yaml` with their default values
- Preserves existing values and structure in your values file
- Provides line number and source file tracking for each reference
- Configurable through options when used as a package
- Atomic file operations ensure values.yaml is never corrupted
- Robust error handling with detailed error messages
- Safe handling of concurrent chart updates

## Error Handling

The tool provides detailed error messages for common issues:
- Invalid chart directory structure
- Missing or inaccessible templates directory
- Permission issues with values.yaml
- Invalid YAML syntax in templates or values
- Concurrent file access conflicts

## Troubleshooting

### Common Issues

1. **Permission Denied**
   - Ensure you have read/write permissions for the chart directory
   - Check file ownership and permissions with `ls -l`

2. **Templates Not Found**
   - Verify your chart has a `templates` directory
   - Check that template files have `.yaml`, `.yml`, or `.tpl` extensions

3. **Values Not Updated**
   - Ensure values.yaml is writable
   - Check for syntax errors in your templates
   - Use verbose mode (-v) to see what references were found

4. **Concurrent Access**
   - The tool uses atomic file operations to prevent corruption
   - If you see "file busy" errors, wait and try again

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

### Development Setup

1. Clone the repository
2. Install Devbox (recommended):
   ```bash
   make install-devbox
   ```
3. Start Devbox shell:
   ```bash
   make devbox
   ```
4. Install dependencies:
   ```bash
   make install
   ```
5. Run all checks:
   ```bash
   make check
   ```

### Development Commands

```bash
# Format code
make fmt

# Run tests
make test

# Run tests with coverage
make coverage

# Run linting
make lint

# Run all checks (vet, lint, test)
make check

# Build the binary
make build

# Generate documentation
make generate

# Clean up temporary files
make clean
```

### Code Style

- Follow standard Go code style and conventions
- Add tests for new functionality
- Update documentation for API changes
- Use meaningful commit messages
