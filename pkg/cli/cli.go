// This package acts as the user interface for interacting with the platform. It handles function deployment
// (compiling the function, building a Docker image, and registering it with the server) and function
// invocation (sending requests to trigger exectuion). It's the entry point for users to interact
// via commands like `serverless deploy` and `serverless invoke`.
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

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

// deployFunction handles the deployment of a user function.
// It compiles the function, builds the Docker image, and registers it with the server.
func deployFunction(name string, config Config, log *logrus.Logger) error {
	// Validate that the function directory exists
	functionDir := filepath.Join("functions", name)
	if _, err := os.Stat(functionDir); os.IsNotExist(err) {
		return fmt.Errorf("function directory %s does not exist", functionDir)
	}

	// Compile the function into a binary
	binaryPath := filepath.Join(functionDir, "function")
	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	cmd.Dir = functionDir
	cmd.Stderr = os.Stderr // Forward compilation errors to user
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to compile function: %v", err)
	}
	log.WithField("function", name).Info("Function compiled")

	// Create a minimal Dockerfile
	dockerfile := `
FROM golang:1.21
COPY function /app/function
ENTRYPOINT ["/app/function"]
`
	dockerfilePath := filepath.Join(functionDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(dockerfile), 0644); err != nil {
		return fmt.Errorf("failed to create Dockerfile: %v", err)
	}
	log.WithField("function", name).Info("Dockerfile created")

	// Build the Docker image
	imageName := fmt.Sprintf("serverless-%s:latest", name)
	cmd = exec.Command("docker", "build", "-t", imageName, ".")
	cmd.Dir = functionDir
	cmd.Stderr = os.Stderr // Show Docker errors to the user
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build Docker image: %v", err)
	}
	log.WithField("function", name).Info("Docker image built")

	// Register the function with the server via HTTP POST
	metadata := map[string]string{
		"name":    name,
		"image":   imageName,
		"runtime": "go",
	}
	body, _ := json.Marshal(metadata) // Safe to ignore error, as metadata is controlled
	resp, err := http.Post(fmt.Sprintf("http://%s/functions", config.ServerAddr), "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to register function with server: %v", err)
	}
	defer resp.Body.Close()

	// Check server response
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// invokeFunction triggers a function execution by sending an HTTP request.
// It passes the event JSON and return the function's response.
func invokeFunction(name, eventJSON string, config Config, log *logrus.Logger) (string, error) {
	// Validate the event JSON to catch syntax errors
	var event interface{}
	if err := json.Unmarshal([]byte(eventJSON), &event); err != nil {
		return "", fmt.Errorf("invalid event JSON: %v", err)
	}

	// Send HTTP POST request to the server's invoke endpoint
	body := bytes.NewBufferString(eventJSON)
	resp, err := http.Post(fmt.Sprintf("http://%s/invoke/%s", config.ServerAddr, name), "application/json", body)
	if err != nil {
		return "", fmt.Errorf("failed to send invoke request: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	result, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(result))
	}

	log.WithField("function", name).Info("Function invoked successfully")
	return string(result), nil
}
