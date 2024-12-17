package shcv

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"
)

// ValueRef represents a Helm value reference found in templates.
// It tracks where values are used in templates and their default values if specified.
type ValueRef struct {
	// Path is the full dot-notation path to the value (e.g. "gateway.domain")
	Path string
	// DefaultValue is the value specified in the template using the default function
	DefaultValue string
	// SourceFile is the template file where this reference was found
	SourceFile string
	// LineNumber is the line number in the source file where the reference appears
	LineNumber int
}

// ID returns a unique identifier for the value reference
func (v *ValueRef) ID() string {
	return fmt.Sprintf("%s:%d:%s", v.Path, v.LineNumber, v.SourceFile)
}

// ValueFile represents a values file
type ValueFile struct {
	// Path is the path to the values file
	Path string
	// Values contains the values from the values file
	Values map[string]any
	// Changed indicates whether values were modified during processing
	Changed bool
}

// Chart represents a Helm chart structure and manages its values and templates.
// It provides functionality to scan templates for value references and ensure
// all referenced values are properly defined in values.yaml.
type Chart struct {
	// Dir is the root directory of the chart
	Dir string
	// ValuesFiles is the path to values.yaml
	ValuesFiles []ValueFile
	// References tracks all .Values references found in templates
	References []ValueRef
	// Templates lists all discovered template files
	Templates []string
	// config contains the chart processing configuration
	config *config
}

// NewChart creates a new Chart instance for the given directory.
func NewChart(dir string, opts ...Option) (*Chart, error) {
	if dir == "" {
		return nil, fmt.Errorf("invalid chart directory: directory path is empty")
	}

	// Check if directory exists
	if _, err := os.Stat(dir); err != nil {
		return nil, fmt.Errorf("invalid chart directory: %w", err)
	}

	// Create a new config with the given options
	config := newConfig(opts)

	// create a new chart and return it
	chart := &Chart{
		Dir:         dir,
		ValuesFiles: make([]ValueFile, 0),
		References:  make([]ValueRef, 0),
		Templates:   make([]string, 0),
		config:      config,
	}

	// Initialize ValuesFiles with the configured file names
	for _, name := range config.ValuesFileName {
		chart.ValuesFiles = append(chart.ValuesFiles, ValueFile{
			Path:   filepath.Join(dir, name),
			Values: make(map[string]any),
		})
	}

	return chart, nil
}

// LoadValueFiles loads the current values from the value files provided.
// If the file doesn't exist, an empty values map is initialized.
// Returns an error if the file exists but cannot be read or parsed.
func (c *Chart) LoadValueFiles() error {
	// iterate over all values files
	for i := range c.ValuesFiles {
		file := &c.ValuesFiles[i] // Get pointer to existing ValueFile
		data, err := os.ReadFile(file.Path)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("reading values file: %w", err)
		}

		// Initialize the values map if nil
		if file.Values == nil {
			file.Values = make(map[string]any)
		}

		// if the file has data lets unmarshal it into the values map
		if len(data) > 0 {
			if err := yaml.Unmarshal(data, &file.Values); err != nil {
				return fmt.Errorf("parsing values file: %w", err)
			}
			if c.config.Verbose {
				fmt.Printf("loaded values from %s\n", file.Path)
			}
		} else {
			if c.config.Verbose {
				fmt.Printf("no values found in %s\n", file.Path)
			}
		}
	}

	return nil
}

// FindTemplates discovers all template files in the chart's templates directory.
// It looks for files with .yaml, .yml, or .tpl extensions.
// Returns an error if the templates directory cannot be accessed.
func (c *Chart) FindTemplates() error {
	// get the full path to the templates directory
	dir := filepath.Join(c.Dir, c.config.TemplatesDir)

	// check if the directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("templates directory not found: %w", err)
	}

	// walk the templates directory and find all template files
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && (strings.HasSuffix(path, ".yaml") ||
			strings.HasSuffix(path, ".yml") ||
			strings.HasSuffix(path, ".tpl")) {
			c.Templates = append(c.Templates, path)
		}
		return nil
	})
}

// ParseTemplates scans all discovered templates for .Values references.
// It identifies both simple references and those with default values.
// The references are stored in the Chart's References slice.
func (c *Chart) ParseTemplates() error {
	// iterate over all templates
	for _, template := range c.Templates {

		// Open the template file and defer closing it
		file, err := os.Open(template)
		if err != nil {
			return fmt.Errorf("opening template %s: %w", template, err)
		}
		defer file.Close()

		// Create a scanner for efficient reading
		scanner := bufio.NewScanner(file)
		var content strings.Builder
		for scanner.Scan() { // read each line of the template
			content.WriteString(scanner.Text()) // append the line to the content
			content.WriteString("\n")           // append a newline to the end of the line
		}

		// Check for any errors from the scanner
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("scanning template %s: %w", template, err)
		}

		// Parse the template content
		if c.config.Verbose {
			fmt.Printf("parsing template %s\n", template)
		}

		// Parse the template content
		refs := ParseFile(content.String(), template)

		// Apply the references to the chart
		c.References = append(c.References, refs...)
	}
	return nil
}

// defaultDeploymentStrategy represents the default deployment strategy configuration
var defaultDeploymentStrategy = map[string]interface{}{
	"type": "RollingUpdate",
	"rollingUpdate": map[string]interface{}{
		"maxSurge":       1,
		"maxUnavailable": 0,
	},
}

// ProcessReferences ensures all referenced values exist in values.yaml.
func (c *Chart) ProcessReferences() {
	// First pass: process deployment strategy for deployment manifests
	for _, template := range c.Templates {
		if err := c.injectDeploymentStrategy(template); err != nil && c.config.Verbose {
			fmt.Printf("warning: failed to process deployment strategy for %s: %v\n", template, err)
		}
	}

	processedRefs := make(map[string]bool) // track processed references paths
	templateRefs := make([]ValueRef, 0)    // final list of references to update

	// Second pass: collect all references and find default values
	for _, ref := range c.References {
		// Skip if we've already processed this reference
		if processedRefs[ref.Path] {
			continue
		}

		// iterate over all references with the same path
		// and find the first default value if it exists
		for _, r := range c.References {
			if ref.Path == r.Path && r.DefaultValue != "" {
				ref.DefaultValue = r.DefaultValue
				break
			}
		}

		// Add this reference to the final list and mark as processed
		templateRefs = append(templateRefs, ref)
		processedRefs[ref.Path] = true
	}

	// Third pass: process all other references
	for i := range c.ValuesFiles {
		file := &c.ValuesFiles[i] // Get pointer to existing ValueFile

		// iterate over each template reference
		for _, ref := range templateRefs {
			// Only set the value if it doesn't already exist or has a default value
			if !valueExists(file.Values, ref.Path) {
				setNestedValue(file.Values, ref.Path, ref.DefaultValue)
				file.Changed = true
			}
		}
	}
}

// injectDeploymentStrategy detects if a template is a Kubernetes Deployment and injects strategy values
func (c *Chart) injectDeploymentStrategy(templatePath string) error {
	content, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("reading template: %w", err)
	}

	// Quick check if this might be a deployment
	if !bytes.Contains(content, []byte("kind: Deployment")) {
		return nil
	}

	// Parse YAML to confirm it's a deployment
	// First, remove Helm template directives that might interfere with YAML parsing
	cleanContent := removeHelmTemplates(content)

	var manifest struct {
		Kind string `yaml:"kind"`
	}
	if err := yaml.Unmarshal(cleanContent, &manifest); err != nil {
		return fmt.Errorf("parsing manifest: %w", err)
	}

	if manifest.Kind != "Deployment" {
		return nil
	}

	if c.config.Verbose {
		fmt.Printf("Found deployment manifest in %s\n", templatePath)
	}

	// Add deployment strategy values if they don't exist
	for i := range c.ValuesFiles {
		file := &c.ValuesFiles[i]

		// Initialize values map if needed
		if file.Values == nil {
			file.Values = make(map[string]interface{})
		}

		if c.config.Verbose {
			fmt.Printf("Processing values file: %s\n", file.Path)
			fmt.Printf("Current values: %+v\n", file.Values)
		}

		// Get or create deployment map while preserving existing structure
		var deployment map[string]interface{}
		if existingDeployment, ok := file.Values["deployment"]; ok {
			if c.config.Verbose {
				fmt.Printf("Found existing deployment section: %+v\n", existingDeployment)
			}
			if deploymentMap, ok := existingDeployment.(map[string]interface{}); ok {
				deployment = deploymentMap
			} else {
				deployment = make(map[string]interface{})
				file.Values["deployment"] = deployment
			}
		} else {
			deployment = make(map[string]interface{})
			file.Values["deployment"] = deployment
		}

		// Check if strategy exists
		if _, hasStrategy := deployment["strategy"]; !hasStrategy {
			if c.config.Verbose {
				fmt.Printf("Adding strategy section to deployment\n")
			}
			// Create a deep copy of defaultDeploymentStrategy
			strategy := make(map[string]interface{})
			for k, v := range defaultDeploymentStrategy {
				if m, ok := v.(map[string]interface{}); ok {
					// Deep copy nested map
					strategy[k] = make(map[string]interface{})
					for k2, v2 := range m {
						strategy[k].(map[string]interface{})[k2] = v2
					}
				} else {
					strategy[k] = v
				}
			}
			deployment["strategy"] = strategy
			file.Changed = true

			if c.config.Verbose {
				fmt.Printf("Updated deployment section: %+v\n", deployment)
			}

			// Only update the template if we added new values
			updatedContent := updateDeploymentTemplate(content)
			if err := os.WriteFile(templatePath, updatedContent, 0644); err != nil {
				return fmt.Errorf("updating template: %w", err)
			}
		} else if c.config.Verbose {
			fmt.Printf("Strategy section already exists\n")
		}
	}

	return nil
}

// removeHelmTemplates removes Helm template directives from YAML content
func removeHelmTemplates(content []byte) []byte {
	lines := strings.Split(string(content), "\n")
	var cleanLines []string

	for _, line := range lines {
		// Skip lines with Helm template directives
		if strings.Contains(line, "{{") || strings.Contains(line, "}}") {
			continue
		}
		// Skip lines with Helm template comments
		if strings.Contains(line, "{{-") || strings.Contains(line, "-}}") {
			continue
		}
		cleanLines = append(cleanLines, line)
	}

	return []byte(strings.Join(cleanLines, "\n"))
}

// updateDeploymentTemplate adds the strategy configuration to a deployment template
func updateDeploymentTemplate(content []byte) []byte {
	// Split the content into lines
	lines := strings.Split(string(content), "\n")

	// Find the spec: line and its indentation
	specIndex := -1
	specIndent := ""
	strategyExists := false
	inSpec := false
	inTemplate := false
	templateDepth := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track template section depth
		if strings.Contains(line, "template:") {
			templateDepth++
			if templateDepth == 1 {
				inTemplate = true
			}
			continue
		}

		// Track when we're in the main spec section
		if trimmed == "spec:" {
			if templateDepth == 0 {
				specIndex = i
				specIndent = line[:len(line)-len(trimmed)]
				inSpec = true
			}
			continue
		}

		// Only look for strategy within the main spec section
		if inSpec && !inTemplate {
			if strings.HasPrefix(trimmed, "strategy:") {
				strategyExists = true
				break
			}
			// If we hit a line with less indentation than spec, we're out of the main spec
			if len(line) > 0 {
				currentIndent := line[:len(line)-len(trimmed)]
				if len(currentIndent) <= len(specIndent) {
					inSpec = false
				}
			}
		}

		// Track template section depth
		if inTemplate {
			currentIndent := len(line) - len(strings.TrimLeft(line, " "))
			if currentIndent <= len(specIndent) {
				templateDepth--
				if templateDepth == 0 {
					inTemplate = false
				}
			}
		}
	}

	// If strategy already exists or we can't find spec, return unchanged
	if strategyExists || specIndex == -1 {
		return content
	}

	// Find the indentation of the first item under spec
	baseIndent := ""
	indentWidth := 2 // Default indent width
	for i := specIndex + 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "{{") {
			continue
		}
		if len(line) > len(trimmed) {
			baseIndent = line[:len(line)-len(trimmed)]
			indentWidth = len(baseIndent) - len(specIndent)
			break
		}
	}
	if baseIndent == "" {
		baseIndent = specIndent + strings.Repeat(" ", indentWidth)
	}

	// Create the strategy section with proper indentation
	strategySection := []string{
		baseIndent + "strategy:",
		baseIndent + strings.Repeat(" ", indentWidth) + "type: {{ .Values.deployment.strategy.type }}",
		baseIndent + strings.Repeat(" ", indentWidth) + "rollingUpdate:",
		baseIndent + strings.Repeat(" ", indentWidth*2) + "maxSurge: {{ .Values.deployment.strategy.rollingUpdate.maxSurge }}",
		baseIndent + strings.Repeat(" ", indentWidth*2) + "maxUnavailable: {{ .Values.deployment.strategy.rollingUpdate.maxUnavailable }}",
	}

	// Insert the strategy section right after spec:
	result := make([]string, 0, len(lines)+len(strategySection))
	result = append(result, lines[:specIndex+1]...)
	result = append(result, strategySection...)
	result = append(result, lines[specIndex+1:]...)

	return []byte(strings.Join(result, "\n"))
}

// UpdateValueFiles ensures all referenced values exist in values.yaml.
// It adds missing values with appropriate defaults and updates the file.
// The operation is skipped if no changes are needed.
func (c *Chart) UpdateValueFiles() error {
	// iterate over each values file
	for i := range c.ValuesFiles {
		file := &c.ValuesFiles[i]
		if !file.Changed {
			continue
		}

		// Convert to YAML with proper formatting
		data, err := yaml.Marshal(file.Values)
		if err != nil {
			return fmt.Errorf("encoding values: %w", err)
		}

		// Write the formatted YAML to file
		if err := os.WriteFile(file.Path, data, 0644); err != nil {
			return fmt.Errorf("writing values file: %w", err)
		}

		if c.config.Verbose {
			fmt.Printf("updated values in %s\n", file.Path)
		}
	}

	return nil
}

// setNestedValue sets a nested value in the Values map
func setNestedValue(values map[string]any, path string, value string) {
	parts := strings.Split(path, ".")
	current := values

	// Create nested structure
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		if _, exists := current[part]; !exists {
			current[part] = make(map[string]any)
		}
		if nested, ok := current[part].(map[string]any); ok {
			current = nested
		} else {
			// Convert existing value to map if needed
			newMap := make(map[string]any)
			current[part] = newMap
			current = newMap
		}
	}

	// Set the final value (remove string conversion)
	current[parts[len(parts)-1]] = value
}

// valueExists is a function to check if a value exists in the values map at the given path
func valueExists(values map[string]any, path string) bool {
	current := values
	parts := strings.Split(path, ".")

	for i, part := range parts {
		v, ok := current[part]
		if !ok {
			return false
		}
		if i == len(parts)-1 {
			return true
		}
		current, ok = v.(map[string]any)
		if !ok {
			return false
		}
	}
	return true
}
