package shcv

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Version is the current version of the shcv package
const Version = "1.0.5"

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

	// Create a new config, apply options, and update values files
	config := defaultConfig()
	config = applyOptions(config, opts)

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

// ProcessReferences ensures all referenced values exist in values.yaml.
func (c *Chart) ProcessReferences() {
	processedRefs := make(map[string]bool) // track processed references paths
	templateRefs := make([]ValueRef, 0)    // final list of references to update

	// Update values files with defaults
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

	// iterate over each values file
	for i := range c.ValuesFiles {
		file := &c.ValuesFiles[i] // Get pointer to existing ValueFile

		// iterate over each reference
		for _, ref := range templateRefs {
			// Always set the value, whether it exists or not
			setNestedValue(file.Values, ref.Path, ref.DefaultValue)
			file.Changed = true
		}
	}
}

// UpdateValueFiles ensures all referenced values exist in values.yaml.
// It adds missing values with appropriate defaults and updates the file atomically.
// The operation is skipped if no changes are needed.
func (c *Chart) UpdateValueFiles() error {
	// iterate over each values file
	for i := range c.ValuesFiles {
		file := &c.ValuesFiles[i] // Get pointer to existing ValueFile
		if !file.Changed {
			continue
		}

		// Write updated values to files
		data, err := yaml.Marshal(file.Values)
		if err != nil {
			return fmt.Errorf("marshaling values: %w", err)
		}

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

// Helper function to check if a value exists at the given path
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
