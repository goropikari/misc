# Dev Container JSON Merger (Go Implementation)

This tool provides commands to merge and edit `devcontainer` override configurations.

## Usage

```bash
go run ./go_impl/cmd/main.go <command> [flags]
# Or, if built: devcontainer-merger-go <command> [flags]
```

### Commands:

- **`merge`**: Merges a base `devcontainer.json` (or JSONC) with an optional `devcontainer.override.json` file.
  ```bash
  go run ./go_impl/cmd/main.go merge [--base-config <path>] [--project-id <id>] [--override-config <path>] [--output-file <path>]
  ```
- **`edit`**: Edits a `devcontainer.override.json` file. It will create the file and necessary directories if they don't exist, and open it with the editor specified in your `EDITOR` environment variable.
  ```bash
  go run ./go_impl/cmd/main.go edit [--project-id <id>] [--override-config <path>]
  ```

### Default Paths & Override Resolution:

The tool resolves the `devcontainer.override.json` file based on the following priority:

1. **Explicit Path**: Use the path provided via the `--override-config` flag.
2. **Project-Specific Default**: If `--override-config` is not given but `--project-id` is, the tool looks for `~/.config/devcontainer_merger/<sanitized_project_id>/override.json`.
3. **Global Default**: If neither of the above yields a file, the tool checks for `~/.config/devcontainer_merger/override.json`.
4. **Creation Fallback (for `edit` command)**: If no override file is found, the `edit` command will create either a project-specific or global default override file, depending on whether `--project-id` is provided.

- **Base config** (for `merge` command): Defaults to `./.devcontainer/devcontainer.json` (relative to current directory).
- **Output file** (for `merge` command): Defaults to `merged-devcontainer.json` in the same directory as the base config.

### Flags:

#### For `merge` command:

- `--base-config`: Path to the base configuration file. Defaults to `./.devcontainer/devcontainer.json`.
- `--project-id`: Identifier for the project to derive a project-specific override path.
- `--override-config`: Explicit path to the override configuration file. If provided, `--project-id` is ignored for override resolution.
- `--output-file`: Path for the merged output file.

#### For `edit` command:

- `--project-id`: Identifier for the project to derive a project-specific override path. If not provided and `--override-config` is also not provided, edits the global default override file.
- `--override-config`: Explicit path to the `devcontainer.override.json` file to edit. If provided, `--project-id` is ignored for file resolution.

## Merging Logic

Follows the specification defined in `SPECIFICATION.md`. Key aspects include:

- JSONC parsing for input files.
- Deep merging for nested objects (including `features`).
- Intelligent merging for specific arrays like `extensions`, and `appPort`.
- Concatenation for command arrays (`postCreateCommand`, etc.).
- Overriding for other array types and primitive types.
- Output is standard JSON.
