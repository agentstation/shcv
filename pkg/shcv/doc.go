/*
Package shcv provides functionality to synchronize Helm chart values by analyzing
template files and updating values files accordingly.

The package helps maintain Helm charts by automatically detecting all {{ .Values.* }}
expressions in template files and ensuring they are properly defined in the values files.
It uses atomic file operations to ensure data integrity and provides robust error handling.

Basic usage:

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

Configuration options:

	chart, err := shcv.NewChart("./my-chart",
		shcv.WithValuesFileNames([]string{"values.yaml", "values-prod.yaml"}),
		shcv.WithTemplatesDir("custom-templates"),
		shcv.WithVerbose(true),
	)

Key features:
  - Detects all Helm value references in template files
  - Supports multiple values files
  - Supports nested value structures
  - Handles default values in templates
  - Creates missing values with their default values
  - Preserves existing values, structure, and data types (e.g., numbers, strings)
  - Provides line number and source file tracking
  - Uses atomic file operations
  - Provides robust error handling

Error handling:
  - Invalid chart directory structure
  - Missing or inaccessible templates directory
  - Permission issues with values files
  - Invalid YAML syntax
  - Concurrent file access conflicts

Requirements:
  - Go 1.21 or later
  - Valid Helm chart directory structure
  - Read/write permissions for chart directory
*/
package shcv
