package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/wazeos/wazeos/cmd/wazeos/commands/apps"
	"github.com/wazeos/wazeos/cmd/wazeos/commands/drivers"
	"github.com/wazeos/wazeos/cmd/wazeos/commands/install"
	"github.com/wazeos/wazeos/cmd/wazeos/commands/secrets"
)

var (
	cfgFile  string
	dataPath string
	verbose  bool
	quiet    bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "wazeos",
	Short: "WazeOS - WebAssembly FaaS Platform",
	Long: `WazeOS is a WebAssembly-native Function-as-a-Service (FaaS) platform
that executes user-defined WASM applications with automatic credential injection
and comprehensive audit logging.

Complete documentation is available at https://github.com/wazeos/wazeos`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.wazeos/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&dataPath, "data-path", "", "data directory path (default: $HOME/.wazeos/data)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "quiet mode (suppress non-error output)")

	// Bind flags to viper
	viper.BindPFlag("data_path", rootCmd.PersistentFlags().Lookup("data-path"))
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	viper.BindPFlag("quiet", rootCmd.PersistentFlags().Lookup("quiet"))

	// Add commands
	rootCmd.AddCommand(apps.AppsCmd)
	rootCmd.AddCommand(drivers.DriversCmd)
	rootCmd.AddCommand(secrets.SecretsCmd)
	rootCmd.AddCommand(install.InstallCmd)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		// Search config in home directory with name ".wazeos" (without extension)
		viper.AddConfigPath(home + "/.wazeos")
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
	}

	// Environment variables
	viper.SetEnvPrefix("WAZEOS")
	viper.AutomaticEnv()

	// Set defaults
	home, _ := os.UserHomeDir()
	defaultDataPath := "./data"
	if home != "" {
		defaultDataPath = home + "/.wazeos/data"
	}
	viper.SetDefault("data_path", defaultDataPath)
	viper.SetDefault("server.addr", ":8081")
	viper.SetDefault("output.format", "table")
	viper.SetDefault("output.color", true)

	// If a config file is found, read it in
	if err := viper.ReadInConfig(); err == nil && !quiet {
		if verbose {
			fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
		}
	}
}
