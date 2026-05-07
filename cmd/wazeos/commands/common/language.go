package common

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Language represents a supported programming language
type Language string

const (
	LanguageGo   Language = "go"
	LanguageRust Language = "rust"
)

// DetectLanguage determines the language of a project based on files present
func DetectLanguage(dir string) (Language, error) {
	// Check for Rust files
	if _, err := os.Stat(filepath.Join(dir, "Cargo.toml")); err == nil {
		return LanguageRust, nil
	}
	if _, err := os.Stat(filepath.Join(dir, "src", "main.rs")); err == nil {
		return LanguageRust, nil
	}
	if _, err := os.Stat(filepath.Join(dir, "main.rs")); err == nil {
		return LanguageRust, nil
	}

	// Check for Go files
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		return LanguageGo, nil
	}
	if _, err := os.Stat(filepath.Join(dir, "main.go")); err == nil {
		return LanguageGo, nil
	}

	return "", fmt.Errorf("unable to detect language: no main.go, main.rs, or Cargo.toml found")
}

// CheckToolchain verifies the required build toolchain is installed
func CheckToolchain(lang Language) error {
	switch lang {
	case LanguageGo:
		cmd := exec.Command("tinygo", "version")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("TinyGo not found. Install it from: https://tinygo.org/getting-started/install/")
		}
		return nil

	case LanguageRust:
		// Check for cargo
		cmd := exec.Command("cargo", "--version")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("Cargo not found. Install Rust from: https://rustup.rs/")
		}

		// Check for wasm32-wasi target
		cmd = exec.Command("rustup", "target", "list", "--installed")
		output, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("rustup not found. Install Rust from: https://rustup.rs/")
		}

		// Check if wasm32-wasi is in the output
		if !contains(string(output), "wasm32-wasi") {
			return fmt.Errorf("wasm32-wasi target not installed. Run: rustup target add wasm32-wasi")
		}

		return nil

	default:
		return fmt.Errorf("unsupported language: %s", lang)
	}
}

// BuildWASM compiles source code to WASM based on language
func BuildWASM(lang Language, dir, outputFile string) error {
	switch lang {
	case LanguageGo:
		return buildGoWASM(dir, outputFile)
	case LanguageRust:
		return buildRustWASM(dir, outputFile)
	default:
		return fmt.Errorf("unsupported language: %s", lang)
	}
}

// buildGoWASM compiles Go code to WASM using TinyGo
func buildGoWASM(dir, outputFile string) error {
	mainFile := filepath.Join(dir, "main.go")

	cmd := exec.Command("tinygo", "build",
		"-o", outputFile,
		"-target=wasi",
		mainFile)
	cmd.Dir = dir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("TinyGo compilation failed:\n%s", string(output))
	}

	return nil
}

// buildRustWASM compiles Rust code to WASM using Cargo
func buildRustWASM(dir, outputFile string) error {
	// Build with cargo
	cmd := exec.Command("cargo", "build",
		"--target", "wasm32-wasi",
		"--release")
	cmd.Dir = dir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Cargo compilation failed:\n%s", string(output))
	}

	// Determine the package name from Cargo.toml to find the output file
	// For simplicity, we'll look for any .wasm file in target/wasm32-wasi/release/
	releaseDir := filepath.Join(dir, "target", "wasm32-wasi", "release")
	entries, err := os.ReadDir(releaseDir)
	if err != nil {
		return fmt.Errorf("failed to read release directory: %w", err)
	}

	// Find the .wasm file (not .wasm.d files)
	var wasmFile string
	for _, entry := range entries {
		name := entry.Name()
		if filepath.Ext(name) == ".wasm" && !entry.IsDir() {
			wasmFile = filepath.Join(releaseDir, name)
			break
		}
	}

	if wasmFile == "" {
		return fmt.Errorf("no .wasm file found in %s", releaseDir)
	}

	// Copy to the desired output location
	input, err := os.ReadFile(wasmFile)
	if err != nil {
		return fmt.Errorf("failed to read WASM file: %w", err)
	}

	if err := os.WriteFile(outputFile, input, 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	return nil
}

// GetSourceFile returns the main source file path for a language
func GetSourceFile(lang Language, dir string) string {
	switch lang {
	case LanguageGo:
		return filepath.Join(dir, "main.go")
	case LanguageRust:
		// Check src/main.rs first, then main.rs
		srcPath := filepath.Join(dir, "src", "main.rs")
		if _, err := os.Stat(srcPath); err == nil {
			return srcPath
		}
		return filepath.Join(dir, "main.rs")
	default:
		return ""
	}
}

// GetToolchainName returns the human-readable name of the toolchain
func GetToolchainName(lang Language) string {
	switch lang {
	case LanguageGo:
		return "TinyGo"
	case LanguageRust:
		return "Cargo"
	default:
		return "Unknown"
	}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		containsInner(s, substr)))
}

func containsInner(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
