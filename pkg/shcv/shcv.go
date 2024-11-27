package shcv

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Version is the current version of the shcv package
const Version = "1.0.0"

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

// Chart represents a Helm chart structure and manages its values and templates.
// It provides functionality to scan templates for value references and ensure
// all referenced values are properly defined in values.yaml.
type Chart struct {
	// Dir is the root directory of the chart
	Dir string
	// ValuesFile is the path to values.yaml
	ValuesFile string
	// Values contains the current values from values.yaml
	Values map[string]any
	// References tracks all .Values references found in templates
	References []ValueRef
	// Templates lists all discovered template files
	Templates []string
	// Changed indicates whether values were modified during processing
	Changed bool
	// options contains the chart processing configuration
	options *Options
}

// Regular expressions for parsing Helm templates
var (
	// Matches {{ .Values.something }} or {{ .Values.something.nested }}
	valueRegex = regexp.MustCompile(`{{\s*\.Values\.([^}\s|]+)`)

	// Matches default values: {{ .Values.something | default "value" }}
	defaultRegex = regexp.MustCompile(`{{\s*\.Values\.([^}\s|]+)\s*\|\s*default\s*"([^"]+)"`)

	// Matches default values with single quotes: {{ .Values.something | default 'value' }}
	defaultSingleQuoteRegex = regexp.MustCompile(`{{\s*\.Values\.([^}\s|]+)\s*\|\s*default\s*'([^']+)'`)

	// Matches numeric default values: {{ .Values.something | default 80 }}
	defaultNumericRegex = regexp.MustCompile(`{{\s*\.Values\.([^}\s|]+)\s*\|\s*default\s+(\d+)`)
)

// Options configures the behavior of Chart processing.
// It allows customization of file locations and default values.
type Options struct {
	// ValuesFileName is the name of the values file to use (default: "values.yaml")
	ValuesFileName string
	// TemplatesDir is the name of the templates directory (default: "templates")
	TemplatesDir string
	// DefaultValues provides default values for specific key patterns
	DefaultValues map[string]string
}

// DefaultOptions returns the default configuration options for Chart processing.
// This includes standard file locations and common default values.
func DefaultOptions() *Options {
	return &Options{
		ValuesFileName: "values.yaml",
		TemplatesDir:   "templates",
		DefaultValues: map[string]string{
			"domain":   "api.example.com",
			"port":     "80",
			"replicas": "1",
			"enabled":  "true",
		},
	}
}

// NewChart creates a new Chart instance for processing a Helm chart.
// It validates the chart directory and initializes the chart with the given options.
// If opts is nil, default options are used.
func NewChart(dir string, opts *Options) (*Chart, error) {
	// Validate directory
	if dir == "" {
		return nil, fmt.Errorf("chart directory cannot be empty")
	}

	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("invalid chart directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path %s is not a directory", dir)
	}

	if opts == nil {
		opts = DefaultOptions()
	} else {
		// Merge with default options
		defaultOpts := DefaultOptions()
		if opts.ValuesFileName == "" {
			opts.ValuesFileName = defaultOpts.ValuesFileName
		}
		if opts.TemplatesDir == "" {
			opts.TemplatesDir = defaultOpts.TemplatesDir
		}
		if opts.DefaultValues == nil {
			opts.DefaultValues = defaultOpts.DefaultValues
		}
	}

	valuesFile := filepath.Join(dir, opts.ValuesFileName)

	return &Chart{
		Dir:        dir,
		ValuesFile: valuesFile,
		Values:     make(map[string]any),
		References: make([]ValueRef, 0),
		Templates:  make([]string, 0),
		options:    opts,
	}, nil
}

// LoadValues loads the current values from the values.yaml file.
// If the file doesn't exist, an empty values map is initialized.
// Returns an error if the file exists but cannot be read or parsed.
func (c *Chart) LoadValues() error {
	data, err := os.ReadFile(c.ValuesFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading values file: %w", err)
	}

	if len(data) > 0 {
		if err := yaml.Unmarshal(data, &c.Values); err != nil {
			return fmt.Errorf("parsing values file: %w", err)
		}
	}

	return nil
}

// FindTemplates discovers all template files in the chart's templates directory.
// It looks for files with .yaml, .yml, or .tpl extensions.
// Returns an error if the templates directory cannot be accessed.
func (c *Chart) FindTemplates() error {
	templatesDir := filepath.Join(c.Dir, c.options.TemplatesDir)
	if _, err := os.Stat(templatesDir); os.IsNotExist(err) {
		return fmt.Errorf("templates directory not found: %w", err)
	}
	return filepath.WalkDir(templatesDir, func(path string, d fs.DirEntry, err error) error {
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
	for _, template := range c.Templates {
		content, err := os.ReadFile(template)
		if err != nil {
			return fmt.Errorf("reading template %s: %w", template, err)
		}

		lines := strings.Split(string(content), "\n")
		for lineNum, line := range lines {
			// Check for values with double-quoted defaults
			matches := defaultRegex.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				c.References = append(c.References, ValueRef{
					Path:         match[1],
					DefaultValue: match[2],
					SourceFile:   template,
					LineNumber:   lineNum + 1,
				})
			}

			// Check for values with single-quoted defaults
			matches = defaultSingleQuoteRegex.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				c.References = append(c.References, ValueRef{
					Path:         match[1],
					DefaultValue: match[2],
					SourceFile:   template,
					LineNumber:   lineNum + 1,
				})
			}

			// Check for values with numeric defaults
			matches = defaultNumericRegex.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				// Convert numeric default to string
				c.References = append(c.References, ValueRef{
					Path:         match[1],
					DefaultValue: fmt.Sprintf("%s", match[2]), // Ensure numeric values are stored as strings
					SourceFile:   template,
					LineNumber:   lineNum + 1,
				})
			}

			// Check for values without defaults
			matches = valueRegex.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				// Skip if we already found this with a default value
				found := false
				for _, ref := range c.References {
					if ref.Path == match[1] && ref.SourceFile == template && ref.LineNumber == lineNum+1 {
						found = true
						break
					}
				}
				if !found {
					c.References = append(c.References, ValueRef{
						Path:       match[1],
						SourceFile: template,
						LineNumber: lineNum + 1,
					})
				}
			}
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

	// Set the final value
	// Always store as string to ensure consistent types
	current[parts[len(parts)-1]] = value
}

// UpdateValues ensures all referenced values exist in values.yaml.
// It adds missing values with appropriate defaults and updates the file atomically.
// The operation is skipped if no changes are needed.
func (c *Chart) UpdateValues() error {
	for _, ref := range c.References {
		// Check if value already exists
		exists := false
		current := c.Values
		parts := strings.Split(ref.Path, ".")

		for i, part := range parts {
			if v, ok := current[part]; ok {
				if i == len(parts)-1 {
					exists = true
					break
				}
				if nested, ok := v.(map[string]any); ok {
					current = nested
				} else {
					break
				}
			} else {
				break
			}
		}

		if !exists {
			// First check custom default values
			lastPart := parts[len(parts)-1]
			value := ""
			if customValue, ok := c.options.DefaultValues[lastPart]; ok {
				value = customValue
			} else if customValue, ok := c.options.DefaultValues[ref.Path]; ok {
				value = customValue
			} else if ref.DefaultValue != "" {
				value = ref.DefaultValue
			} else {
				// Finally check pattern-based defaults
				for pattern, defaultValue := range DefaultOptions().DefaultValues {
					if strings.HasSuffix(lastPart, pattern) {
						value = defaultValue
						break
					}
				}
			}

			if value != "" {
				// Convert numeric values to strings
				if _, err := strconv.Atoi(value); err == nil {
					value = fmt.Sprintf("%s", value)
				}
				setNestedValue(c.Values, ref.Path, value)
				c.Changed = true
			}
		}
	}

	if !c.Changed {
		return nil
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(c.ValuesFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating values directory: %w", err)
	}

	// Convert all numeric values to strings before marshaling
	stringValues := make(map[string]any)
	convertToStrings(c.Values, stringValues)

	// Write updated values back to file
	data, err := yaml.Marshal(stringValues)
	if err != nil {
		return fmt.Errorf("marshaling values: %w", err)
	}

	return os.WriteFile(c.ValuesFile, data, 0644)
}

// convertToStrings recursively converts all numeric values to strings
func convertToStrings(in, out map[string]any) {
	for k, v := range in {
		switch val := v.(type) {
		case map[string]any:
			newMap := make(map[string]any)
			convertToStrings(val, newMap)
			out[k] = newMap
		case int:
			out[k] = fmt.Sprintf("%d", val)
		case float64:
			out[k] = fmt.Sprintf("%g", val)
		default:
			out[k] = v
		}
	}
}
