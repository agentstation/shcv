package main

import (
	"fmt"
	"io"
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
		return processChart(args[0], verbose, cmd.OutOrStdout())
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

func processChart(chartDir string, verbose bool, out io.Writer) error {
	chart, err := shcv.NewChart(chartDir, shcv.WithVerbose(verbose))
	if err != nil {
		return fmt.Errorf("error creating chart: %w", err)
	}

	if err := chart.LoadValueFiles(); err != nil {
		return fmt.Errorf("error loading values: %w", err)
	}

	if err := chart.FindTemplates(); err != nil {
		return fmt.Errorf("error finding templates: %w", err)
	}

	if err := chart.ParseTemplates(); err != nil {
		return fmt.Errorf("error parsing templates: %w", err)
	}

	if verbose {
		fmt.Fprintf(out, "Found %d template files\n", len(chart.Templates))
		fmt.Fprintf(out, "Found %d value references\n", len(chart.References))
		for _, ref := range chart.References {
			fmt.Fprintf(out, "- %s (from %s:%d)\n", ref.Path, filepath.Base(ref.SourceFile), ref.LineNumber)
			if ref.DefaultValue != "" {
				fmt.Fprintf(out, "  default: %s\n", ref.DefaultValue)
			}
		}
		fmt.Fprintln(out)
	}

	chart.ProcessReferences()
	if err := chart.UpdateValueFiles(); err != nil {
		return fmt.Errorf("error updating values: %w", err)
	}

	return nil
}

// osExit is used to mock os.Exit in tests
var osExit = os.Exit

func main() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		osExit(1)
	}
}
