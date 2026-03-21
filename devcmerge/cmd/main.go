package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec" // For executing external commands
	"path/filepath"

	// For strings.ReplaceAll
	"devcmerge/internal/merger" // Assuming merger package is correctly imported from its path
)

// resolveOverridePath encapsulates the logic to find the override file based on priority.
func resolveOverridePath(overrideConfigPathExplicit string) (string, error) {
	// Priority 1: Explicitly provided path
	if overrideConfigPathExplicit != "" {
		return overrideConfigPathExplicit, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("error getting home directory: %w", err)
	}

	// Priority 2: Global Default
	globalDefaultPath := filepath.Join(homeDir, ".config", "devcontainer_merger", "override.json")
	if _, err := os.Stat(globalDefaultPath); err == nil {
		return globalDefaultPath, nil
	}

	return "", nil // No override path resolved
}

func mergeCmd() {
	mergeFlagSet := flag.NewFlagSet("merge", flag.ExitOnError)
	baseConfigPath := mergeFlagSet.String("base-config", "./.devcontainer/devcontainer.json", "Path to the base devcontainer.json. Defaults to './.devcontainer/devcontainer.json'.")
	overrideConfigPathExplicit := mergeFlagSet.String("override-config", "", "Explicit path to the devcontainer.override.json.")
	outputFile := mergeFlagSet.String("output-file", "", "Path for the merged output file. Defaults to 'merged-devcontainer.json' in the same directory as the base config.")

	mergeFlagSet.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s merge:\n", os.Args[0]) // Added \n
		mergeFlagSet.PrintDefaults()
	}

	mergeFlagSet.Parse(os.Args[2:])

	baseConfigPathVal := *baseConfigPath
	overrideConfigPathExplicitVal := *overrideConfigPathExplicit
	outputFileVal := *outputFile

	resolvedOverrideConfigPath, err := resolveOverridePath(overrideConfigPathExplicitVal)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving override path: %v\n", err) // Changed to

		os.Exit(1)
	}

	if resolvedOverrideConfigPath != "" {
		fmt.Printf("Info: Override config resolved to '%s'\n", resolvedOverrideConfigPath) // Changed to
	} else {
		fmt.Println("Info: No override configuration found or specified. Proceeding without overrides.")
	}

	finalOutputFile := outputFileVal
	if finalOutputFile == "" {
		baseDir := filepath.Dir(baseConfigPathVal)
		finalOutputFile = filepath.Join(baseDir, "merged-devcontainer.json")
	}

	baseConfig, err := merger.ParseJSONC(baseConfigPathVal)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("Warning: Base config file not found at '%s'. Creating an empty base for merging.\n", baseConfigPathVal) // Changed to

			baseConfig = make(map[string]any)
		} else {
			fmt.Fprintf(os.Stderr, "Error parsing base config '%s': %v\n", baseConfigPathVal, err) // Changed to

			os.Exit(1)
		}
	}

	overrideConfig := make(map[string]any)
	if resolvedOverrideConfigPath != "" {
		parsedOverride, err := merger.ParseJSONC(resolvedOverrideConfigPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing override config '%s': %v\n", resolvedOverrideConfigPath, err) // Changed to

			os.Exit(1)
		}
		overrideConfig = parsedOverride
	}

	mergedConfig, err := merger.MergeConfigs(baseConfig, overrideConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error merging configurations: %v\n", err) // Changed to

		os.Exit(1)
	}

	mergedJSON, err := json.MarshalIndent(mergedConfig, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling merged config: %v\n", err) // Changed to

		os.Exit(1)
	}

	if err := os.WriteFile(finalOutputFile, mergedJSON, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing output file '%s': %v\n", finalOutputFile, err) // Changed to

		os.Exit(1)
	}

	fmt.Printf("Successfully merged configuration to '%s'\n", finalOutputFile) // Changed to
}

func editCmd() {
	editFlagSet := flag.NewFlagSet("edit", flag.ExitOnError)
	overrideConfigPathExplicit := editFlagSet.String("override-config", "", "Explicit path to the devcontainer.override.json file to edit.")

	editFlagSet.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s edit:\n", os.Args[0])
		editFlagSet.PrintDefaults()
	}

	editFlagSet.Parse(os.Args[2:])

	overrideConfigPathExplicitVal := *overrideConfigPathExplicit

	overrideFilePath, err := resolveOverridePath(overrideConfigPathExplicitVal)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving override path: %v\n", err) // Changed to

		os.Exit(1)
	}

	if overrideFilePath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting home directory: %v\n", err) // Changed to

			os.Exit(1)
		}

		overrideFilePath = filepath.Join(homeDir, ".config", "devcontainer_merger", "override.json")
		fmt.Printf("Info: Creating new global default override file at '%s'\n", overrideFilePath) // Changed to

	} else {
		fmt.Printf("Info: Editing override file at '%s'\n", overrideFilePath) // Changed to
	}

	targetDir := filepath.Dir(overrideFilePath)
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to create directory '%s': %v\n", targetDir, err) // Changed to

			os.Exit(1)
		}
		fmt.Printf("Info: Created directory '%s'\n", targetDir) // Changed to

	}

	if _, err := os.Stat(overrideFilePath); os.IsNotExist(err) {
		emptyJSON := []byte("{}")
		if err := os.WriteFile(overrideFilePath, emptyJSON, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to create empty override file at '%s': %v\n", overrideFilePath, err) // Changed to

			os.Exit(1)
		}
		fmt.Printf("Info: Created empty override file at '%s'\n", overrideFilePath) // Changed to

	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		fmt.Fprintln(os.Stderr, "Error: EDITOR environment variable is not set. Please set it to your preferred editor (e.g., 'EDITOR=code', 'EDITOR=nvim', 'EDITOR=vi').") // Changed to

		os.Exit(1)
	}

	cmd := exec.Command(editor, overrideFilePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Editor command failed: %v\n", err) // Changed to

		os.Exit(1)
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: devcontainer-merger-go <command> [arguments]")
		fmt.Println("Commands:")
		fmt.Println("  merge - Merge devcontainer.json with devcontainer.override.json")
		fmt.Println("  edit  - Edit devcontainer.override.json file")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "merge":
		mergeCmd()
	case "edit":
		editCmd()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1]) // Changed to

		fmt.Println("Usage: devcontainer-merger-go <command> [arguments]")
		fmt.Println("Commands:")
		fmt.Println("  merge - Merge devcontainer.json with devcontainer.override.json")
		fmt.Println("  edit  - Edit devcontainer.override.json file")
		os.Exit(1)
	}
}
