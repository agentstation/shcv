package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/agentstation/shcv/pkg/shcv"
	"github.com/spf13/cobra"
)

// RootCmd is the root command for shcv
var RootCmd = &cobra.Command{
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
		return processChart(args[0], verbose)
	},
	Version: shcv.Version,
}

func init() {
	RootCmd.Flags().BoolP("verbose", "v", false, "verbose output showing all found references")
	RootCmd.SetVersionTemplate(`{{.Version}}
`)

	// Add example usage
	RootCmd.Example = `  # Process chart in current directory
  shcv .

  # Process chart with verbose output
  shcv -v ./my-helm-chart

  # Show version
  shcv --version`
}

func processChart(chartDir string, verbose bool) error {
	chart, err := shcv.NewChart(chartDir, nil) // Use default options
	if err != nil {
		return fmt.Errorf("error creating chart: %w", err)
	}

	if err := chart.LoadValues(); err != nil {
		return fmt.Errorf("error loading values: %w", err)
	}

	if err := chart.FindTemplates(); err != nil {
		return fmt.Errorf("error finding templates: %w", err)
	}

	if err := chart.ParseTemplates(); err != nil {
		return fmt.Errorf("error parsing templates: %w", err)
	}

	if verbose {
		fmt.Printf("Found %d template files\n", len(chart.Templates))
		fmt.Printf("Found %d value references\n", len(chart.References))
		for _, ref := range chart.References {
			fmt.Printf("- %s (from %s:%d)\n", ref.Path, filepath.Base(ref.SourceFile), ref.LineNumber)
			if ref.DefaultValue != "" {
				fmt.Printf("  default: %s\n", ref.DefaultValue)
			}
		}
		fmt.Println()
	}

	if err := chart.UpdateValues(); err != nil {
		return fmt.Errorf("error updating values: %w", err)
	}

	if verbose && chart.Changed {
		fmt.Printf("Successfully updated %s\n", chart.ValuesFile)
	}

	return nil
}

func main() {
	if err := RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
