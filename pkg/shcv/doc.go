/*
Package shcv provides functionality to synchronize Helm chart values by analyzing
template files and updating values.yaml accordingly.

The package helps maintain Helm charts by automatically detecting all {{ .Values.* }}
expressions in template files and ensuring they are properly defined in the values file.
It uses atomic file operations to ensure data integrity and provides robust error handling.

Version: 1.0.4
Requires: Go 1.21 or later

Basic usage:

	import "github.com/agentstation/shcv/pkg/shcv"

	// Create a new chart instance with default options
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

Custom options:

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

Features:
  - Detects all Helm value references in template files
  - Supports nested value structures (e.g., {{ .Values.gateway.domain }})
  - Handles default values in templates (e.g., {{ .Values.domain | default "api.example.com" }})
  - Supports double-quoted, single-quoted, and numeric default values
  - Creates missing values in values.yaml with their default values
  - Preserves existing values and structure in your values file
  - Provides line number and source file tracking for each reference
  - Uses atomic file operations to prevent data corruption
  - Provides robust error handling with detailed messages
  - Safely handles concurrent chart updates

Error Handling:
  - Invalid chart directory structure
  - Missing or inaccessible templates directory
  - Permission issues with values.yaml
  - Invalid YAML syntax in templates or values
  - Concurrent file access conflicts
*/
package shcv
