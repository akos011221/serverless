package cli

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

// Config holds platform configuration, retrieved from YAML.
type Config struct {
	ServerAddr string `yaml:"server_addr"` // HTTP server address
	DBPath     string `yaml:"db_path"`     // SQLite database path
}

// loadConfig reads and parses the YAML configuration file.
func loadConfig(filePath string, log *logrus.Logger) (Config, error) {
	config := Config{
		ServerAddr: "localhost:8080", // Default for server address
		DBPath:     "serverless.db",  // Default for database path
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.WithField("file", filePath).Warn("Config file not found, using defaults")
			return config, nil
		}
		return config, fmt.Errorf("failed to read config file: %v", err)
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("failed to parse config file: %v", err)
	}

	log.WithField("config", config).Info("Configuration loaded")
	return config, nil
}

// RegisterCommands adds CLI commands to the root command.
// It provides modularity by decoupling the CLI logic from the main package.
func RegisterCommands(rootCmd *cobra.Command, configFile string, log *logrus.Logger) {
	config, err := loadConfig(configFile, log)
	if err != nil {
		log.WithError(err).Fatal("Failed to load configuration")
	}

	// Deploy command: `serverless deploy [function-name]`
	// This compiles the function, builds a Docker image, and register it with the server
	deployCmd := &cobra.Command{
		Use:   "deploy [function-name]",
		Short: "Deploy a function to the platform",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			functionName := args[0]
			if err := deployFunction(functionName, config, log); err != nil {
				log.WithError(err).WithField("function", functionName).Fatal("Deploy failed")
			}
			log.WithField("function", functionName).Info("Function deployed successfully")
		},
	}

	// Invoke command: `serverless invoke [function-name] [event-json]`
	// This sends an HTTP request to trigger function execution with the provided event
	invokeCmd := &cobra.Command{
		Use:   "invoke [function-name] [event-json]",
		Short: "Invoke a function with a JSON event",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			functionName := args[0]
			eventJSON := args[1]
			result, err := invokeFunction(functionName, eventJSON, config, log)
			if err != nil {
				log.WithError(err).WithField("function", functionName).Fatal("Invoke failed")
			}
			fmt.Println(result)
		},
	}

	rootCmd.AddCommand(deployCmd, invokeCmd)
}
