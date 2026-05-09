package pkg

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"

	"github.com/pelletier/go-toml/v2"
)

// Manifest represents a wazeos.toml package manifest
type Manifest struct {
	Package     PackageInfo       `toml:"package"`
	Tool        *ToolInfo         `toml:"tool,omitempty"`
	Driver      *DriverInfo       `toml:"driver,omitempty"`
	Permissions PermissionsInfo   `toml:"permissions"`
	Build       BuildInfo         `toml:"build,omitempty"`
	Dependencies map[string]string `toml:"dependencies,omitempty"`
}

// PackageInfo contains basic package metadata
type PackageInfo struct {
	Name        string   `toml:"name"`
	Version     string   `toml:"version"`
	Authors     []string `toml:"authors,omitempty"`
	Description string   `toml:"description,omitempty"`
	License     string   `toml:"license,omitempty"`
	Repository  string   `toml:"repository,omitempty"`
	Keywords    []string `toml:"keywords,omitempty"`
}

// ToolInfo contains MCP tool-specific metadata
type ToolInfo struct {
	Name        string                 `toml:"name"`
	Description string                 `toml:"description"`
	InputSchema map[string]interface{} `toml:"input_schema"`
}

// DriverInfo contains driver-specific metadata
type DriverInfo struct {
	Class        string   `toml:"class"`
	URIPattern   string   `toml:"uri_pattern"`
	Capabilities []string `toml:"capabilities"`
}

// PermissionsInfo declares what resources the package needs access to
type PermissionsInfo struct {
	File  []string `toml:"file,omitempty"`
	HTTP  []string `toml:"http,omitempty"`
	Shell []string `toml:"shell,omitempty"`
	Env   []string `toml:"env,omitempty"`
}

// BuildInfo contains build configuration
type BuildInfo struct {
	Target    string            `toml:"target,omitempty"`
	Release   bool              `toml:"release,omitempty"`
	Features  []string          `toml:"features,omitempty"`
	BuildVars map[string]string `toml:"build_vars,omitempty"`
}

// LoadManifest reads and parses a wazeos.toml file
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest Manifest
	if err := toml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	if err := manifest.Validate(); err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}

	return &manifest, nil
}

// Validate checks if the manifest is valid
func (m *Manifest) Validate() error {
	// Validate package name
	if m.Package.Name == "" {
		return fmt.Errorf("package name is required")
	}
	if !isValidPackageName(m.Package.Name) {
		return fmt.Errorf("invalid package name: must be lowercase alphanumeric with hyphens")
	}

	// Validate version
	if m.Package.Version == "" {
		return fmt.Errorf("package version is required")
	}
	if !isValidSemver(m.Package.Version) {
		return fmt.Errorf("invalid version: must be valid semver (e.g., 1.0.0)")
	}

	// Must be either a tool or a driver, not both
	if m.Tool != nil && m.Driver != nil {
		return fmt.Errorf("package cannot be both a tool and a driver")
	}
	if m.Tool == nil && m.Driver == nil {
		return fmt.Errorf("package must specify either [tool] or [driver]")
	}

	// Validate tool-specific fields
	if m.Tool != nil {
		if m.Tool.Name == "" {
			return fmt.Errorf("tool name is required")
		}
		if m.Tool.Description == "" {
			return fmt.Errorf("tool description is required")
		}
		if m.Tool.InputSchema == nil {
			return fmt.Errorf("tool input_schema is required")
		}
	}

	// Validate driver-specific fields
	if m.Driver != nil {
		if m.Driver.Class == "" {
			return fmt.Errorf("driver class is required")
		}
		if m.Driver.URIPattern == "" {
			return fmt.Errorf("driver uri_pattern is required")
		}
		if len(m.Driver.Capabilities) == 0 {
			return fmt.Errorf("driver must specify at least one capability")
		}
	}

	return nil
}

// IsApp returns true if this is an app/tool manifest
func (m *Manifest) IsApp() bool {
	return m.Tool != nil
}

// IsDriver returns true if this is a driver manifest
func (m *Manifest) IsDriver() bool {
	return m.Driver != nil
}

// ToJSON converts the manifest to JSON (for MCP tool schemas)
func (m *Manifest) ToJSON() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

// GetMCPToolSchema returns the MCP tool schema for this app
func (m *Manifest) GetMCPToolSchema() (map[string]interface{}, error) {
	if !m.IsApp() {
		return nil, fmt.Errorf("not an app manifest")
	}

	return map[string]interface{}{
		"name":        m.Tool.Name,
		"description": m.Tool.Description,
		"inputSchema": m.Tool.InputSchema,
	}, nil
}

// Helper functions

func isValidPackageName(name string) bool {
	// Allow lowercase letters, numbers, and hyphens
	// Must start with a letter
	match, _ := regexp.MatchString(`^[a-z][a-z0-9-]*$`, name)
	return match
}

func isValidSemver(version string) bool {
	// Simple semver validation (major.minor.patch)
	match, _ := regexp.MatchString(`^\d+\.\d+\.\d+(-[a-zA-Z0-9.-]+)?$`, version)
	return match
}

// SaveManifest writes a manifest to a file
func SaveManifest(path string, manifest *Manifest) error {
	data, err := toml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	return nil
}
