package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// OutputConfig holds global output settings
type OutputConfig struct {
	JSON  bool
	Quiet bool
}

var outputCfg OutputConfig

// parseGlobalFlags extracts --json and --quiet from args, returns remaining args
func parseGlobalFlags(args []string) []string {
	var remaining []string
	for _, arg := range args {
		switch arg {
		case "--json":
			outputCfg.JSON = true
		case "--quiet", "-q":
			outputCfg.Quiet = true
		default:
			remaining = append(remaining, arg)
		}
	}
	return remaining
}

// PrintResult outputs data based on output config
func PrintResult(data interface{}) {
	if outputCfg.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(data)
		return
	}

	// For non-JSON, try to print in a human-readable way
	switch v := data.(type) {
	case string:
		fmt.Println(v)
	case []string:
		for _, s := range v {
			fmt.Println(s)
		}
	case map[string]interface{}:
		for k, val := range v {
			fmt.Printf("%s: %v\n", k, val)
		}
	default:
		// Fall back to JSON for complex types
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(data)
	}
}

// PrintTable outputs tabular data
func PrintTable(headers []string, rows [][]string) {
	if outputCfg.JSON {
		result := make([]map[string]string, len(rows))
		for i, row := range rows {
			m := make(map[string]string)
			for j, h := range headers {
				if j < len(row) {
					m[h] = row[j]
				}
			}
			result[i] = m
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(result)
		return
	}

	// Calculate column widths
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Print header
	for i, h := range headers {
		fmt.Printf("%-*s  ", widths[i], h)
	}
	fmt.Println()

	// Print separator
	for i := range headers {
		fmt.Print(strings.Repeat("-", widths[i]), "  ")
	}
	fmt.Println()

	// Print rows
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) {
				fmt.Printf("%-*s  ", widths[i], cell)
			}
		}
		fmt.Println()
	}
}

// PrintProgress prints progress if not quiet or JSON mode
func PrintProgress(format string, args ...interface{}) {
	if !outputCfg.Quiet && !outputCfg.JSON {
		fmt.Printf(format, args...)
	}
}

// PrintInfo prints info message if not quiet
func PrintInfo(format string, args ...interface{}) {
	if !outputCfg.Quiet {
		fmt.Printf(format, args...)
	}
}

// PrintError prints error to stderr
func PrintError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
}
