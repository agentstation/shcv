# shcv (Sync Helm Chart Values)

[![Go Report Card](https://goreportcard.com/badge/github.com/agentstation/shcv)](https://goreportcard.com/report/github.com/agentstation/shcv)
[![GoDoc](https://godoc.org/github.com/agentstation/shcv?status.svg)](https://godoc.org/github.com/agentstation/shcv)
[![GitHub Workflow Status](https://img.shields.io/github/actions/workflow/status/agentstation/uuidkey/ci.yaml?style=flat-square)](https://github.com/agentstation/uuidkey/actions)
[![codecov](https://codecov.io/gh/agentstation/shcv/branch/master/graph/badge.svg?token=7A0O794SOM)](https://codecov.io/gh/agentstation/shcv)
[![License](https://img.shields.io/github/license/agentstation/shcv.svg)](LICENSE)
[![Version](https://img.shields.io/github/v/tag/agentstation/shcv?sort=semver)](https://github.com/agentstation/shcv/releases)
[![Go Version](https://img.shields.io/badge/go-%3E%3D%201.21-blue)](go.mod)

`shcv` is a command-line tool and Go package that helps maintain Helm chart values by automatically synchronizing values files with the parameters used in your Helm templates. It scans all template files for `{{ .Values.* }}` expressions and ensures they are properly defined in your values files.

## Features

- Automatically detects all Helm value references in template files
- Supports multiple values files
- Supports nested value structures (e.g., `{{ .Values.gateway.domain }}`)
- Handles default values in templates (e.g., `{{ .Values.domain | default "api.example.com" }}`)
- Creates missing values in values files with their default values
- Preserves existing values, structure, and data types in your values files
- Provides line number and source file tracking for each reference
- Automatically injects and manages Kubernetes deployment strategies
- Uses atomic file operations to prevent data corruption
- Provides robust error handling with detailed messages

## Installation

```bash
go install github.com/agentstation/shcv@latest
```

## Quick Start

```bash
# Process chart in current directory
shcv .

# Process chart with verbose output
shcv -v ./my-helm-chart

# Show version
shcv --version
```

## Usage

### Command Line Interface

The CLI provides a simple interface to process Helm charts:

```bash
shcv [flags] CHART_DIRECTORY
```

Available flags:
- `-v, --verbose`: Enable verbose output showing all found references
- `--version`: Show version information
- `-h, --help`: Show help information

### Go Package

```go
import "github.com/agentstation/shcv/pkg/shcv"

// Create a new chart instance
chart, err := shcv.NewChart("./my-chart")
if err != nil {
    log.Fatal(err)
}

// Process the chart
if err := chart.LoadValueFiles(); err != nil {
    log.Fatal(err)
}
if err := chart.FindTemplates(); err != nil {
    log.Fatal(err)
}
if err := chart.ParseTemplates(); err != nil {
    log.Fatal(err)
}
if err := chart.ProcessReferences(); err != nil {
    log.Fatal(err)
}
if err := chart.UpdateValueFiles(); err != nil {
    log.Fatal(err)
}
```

### Configuration Options

The package provides functional options for customization:

```go
chart, err := shcv.NewChart("./my-chart",
    shcv.WithValuesFileNames([]string{"values.yaml", "values-prod.yaml"}),
    shcv.WithTemplatesDir("custom-templates"),
    shcv.WithVerbose(true),
)
```

## Example

Given a template file `templates/ingress.yaml`:
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{ .Values.name | default "my-ingress" }}
spec:
  rules:
  - host: {{ .Values.domain | default "example.com" }}
    http:
      paths:
      - path: {{ .Values.path | default "/" }}
        backend:
          service:
            port:
              number: {{ .Values.port | default 80 }}
```

Running `shcv .` will create/update `values.yaml`:
```yaml
name: "my-ingress"
domain: "example.com"
path: "/"
port: 80
```

### Deployment Strategy Example

For Kubernetes deployment manifests, `shcv` automatically injects deployment strategy configuration. Given a template file `templates/deployment.yaml`:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Values.name }}
spec:
  selector:
    matchLabels:
      app: {{ .Values.name }}
  template:
    metadata:
      labels:
        app: {{ .Values.name }}
    spec:
      containers:
      - name: {{ .Values.name }}
        image: {{ .Values.image }}
```

Running `shcv .` will add deployment strategy configuration to `values.yaml`:
```yaml
deployment:
  strategy:
    type: "RollingUpdate"
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
```

And update the deployment template with strategy configuration:
```yaml
spec:
  strategy:
    type: {{ .Values.deployment.strategy.type }}
    rollingUpdate:
      maxSurge: {{ .Values.deployment.strategy.rollingUpdate.maxSurge }}
      maxUnavailable: {{ .Values.deployment.strategy.rollingUpdate.maxUnavailable }}
```

## Requirements

- Go 1.21 or later
- A valid Helm chart directory structure
- Read/write permissions for the chart directory

## Error Handling

The tool provides detailed error messages for:
- Invalid chart directory structure
- Missing or inaccessible templates directory
- Permission issues with values files
- Invalid YAML syntax
- Concurrent file access conflicts

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.