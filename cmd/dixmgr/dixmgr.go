package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

type Config struct {
	Target       string           `toml:"target"`
	Name         string           `toml:"name"`
	DotidxRoot   string           `toml:"dotidx_root"`
	DotidxBackup string           `toml:"dotidx_backup"`
	DotidxRun    string           `toml:"dotidx_run"`
	DotidxLogs   string           `toml:"dotidx_logs"`
	DotidxDB     DotidxDB         `toml:"dotidx_db"`
	Parachains   ParachainsConfig `toml:"parachains"`
	Filesystem   FilesystemConfig `toml:"filesystem"`
	Monitoring   MonitoringConfig `toml:"monitoring"`
}

type DotidxDB struct {
	Type          string   `toml:"type"`
	Version       int      `toml:"version"`
	Name          string   `toml:"name"`
	IP            string   `toml:"ip"`
	User          string   `toml:"user"`
	Port          int      `toml:"port"`
	Memory        string   `toml:"memory"`
	Data          string   `toml:"data"`
	Run           string   `toml:"run"`
	WhitelistedIP []string `toml:"whitelisted_ip"`
}

type ParaChainConfig struct {
	Name            string `toml:"name"`
	Port            int    `toml:"port"`
	Basepath        string `toml:"basepath"`
	ChainreaderIP   string `toml:"chainreader_ip"`
	ChainreaderPort int    `toml:"chainreader_port"`
	SidecarIP       string `toml:"sidecar_ip"`
	SidecarPort     int    `toml:"sidecar_port"`
}

type ParachainsConfig struct {
	Polkadot struct {
		Polkadot    ParaChainConfig `toml:"polkadot"`
		Assethub    ParaChainConfig `toml:"assethub"`
		People      ParaChainConfig `toml:"people"`
		Collectives ParaChainConfig `toml:"collectives"`
	} `toml:"polkadot"`
}

type FilesystemConfig struct {
	ZFS bool `toml:"zfs"`
}

type MonitoringConfig struct {
	User string `toml:"user"`
}

func main() {
	var configFile string
	var configsDir string

	var rootCmd = &cobra.Command{
		Use:   "dixmgr",
		Short: "dixmgr - a tool to manage dotidx configuration",
		Long: `dixmgr is a CLI tool that helps generate and manage configuration for dotidx sites.
It handles complex configuration parameters and generates the necessary configuration files.`,
	}

	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "config file (required)")
	rootCmd.MarkPersistentFlagRequired("config")
	rootCmd.PersistentFlags().StringVarP(&configsDir, "configs-dir", "d", "configsDir", "directory containing template files")

	// Generate command
	generateCmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate configuration files",
		Long:  "Generate configuration files for the site",
		RunE: func(cmd *cobra.Command, args []string) error {
			if configFile == "" {
				return fmt.Errorf("config file is required")
			}

			config, err := loadConfig(configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if err := generateConfig(config, configsDir); err != nil {
				return fmt.Errorf("failed to generate config: %w", err)
			}

			return nil
		},
	}

	rootCmd.AddCommand(generateCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func loadConfig(file string) (*Config, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

func generateConfig(config *Config, configsDir string) error {
	// Create the dist directory with the site name
	distDir := filepath.Join("dist-" + config.Name)
	if err := os.MkdirAll(distDir, 0755); err != nil {
		return fmt.Errorf("failed to create dist directory: %w", err)
	}

	fmt.Printf("Created distribution directory: %s\n", distDir)

	// Copy the config file to the dist directory
	configData, err := toml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(filepath.Join(distDir, "config.toml"), configData, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Check if configs directory exists
	if _, err := os.Stat(configsDir); os.IsNotExist(err) {
		return fmt.Errorf("configs directory does not exist: %s", configsDir)
	}

	// Process all files in the configs directory recursively
	return filepath.Walk(configsDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if path == configsDir {
			return nil
		}

		// Skip files that end with ~ (backup files)
		if strings.HasSuffix(path, "~") {
			fmt.Printf("Skipping backup file: %s\n", path)
			return nil
		}

		// Calculate the relative path from configs dir
		relPath, err := filepath.Rel(configsDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		// Target path in the dist directory
		targetPath := filepath.Join(distDir, relPath)

		if info.IsDir() {
			// Create directory in the dist
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", targetPath, err)
			}
			fmt.Printf("Created directory: %s\n", targetPath)
			return nil
		}

		// Process the file based on its type
		if isTextFile(path) {
			// Process text files as templates
			return processFileAsTemplate(path, targetPath, config)
		} else {
			// Copy binary files directly
			return copyFile(path, targetPath)
		}
	})
}

func isTextFile(path string) bool {
	// Common text file extensions
	textExtensions := []string{
		".txt", ".md", ".json", ".yaml", ".yml", ".toml",
		".conf", ".cfg", ".ini", ".sh", ".bash", ".zsh",
		".py", ".go", ".js", ".html", ".css", ".xml",
		".sql", ".service", ".env", ".template", ".tmpl",
	}

	ext := filepath.Ext(path)
	for _, textExt := range textExtensions {
		if ext == textExt {
			return true
		}
	}

	// Try to detect text files without extension by reading a small sample
	file, err := os.Open(path)
	if err != nil {
		return false // Assume binary if can't open
	}
	defer file.Close()

	// Read the first 512 bytes
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil {
		return false // Assume binary if can't read
	}

	// Check if file might be binary
	for i := 0; i < n; i++ {
		if buffer[i] == 0 {
			return false // Contains null byte, likely binary
		}
	}

	return true // Likely text file
}

func processFileAsTemplate(src, dst string, config *Config) error {
	// Read template file
	tmplData, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read template file %s: %w", src, err)
	}

	// Parse template
	tmpl, err := template.New(filepath.Base(src)).Parse(string(tmplData))
	if err != nil {
		return fmt.Errorf("failed to parse template %s: %w", src, err)
	}

	// Create output file
	outFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create output file %s: %w", dst, err)
	}
	defer outFile.Close()

	// Execute template
	if err := tmpl.Execute(outFile, config); err != nil {
		return fmt.Errorf("failed to execute template %s: %w", src, err)
	}

	fmt.Printf("Generated %s from template %s\n", dst, src)
	return nil
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", src, err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", dst, err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy file contents from %s to %s: %w", src, dst, err)
	}

	fmt.Printf("Copied %s to %s\n", src, dst)
	return nil
}
