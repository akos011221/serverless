package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/akos011221/serverless/pkg/cli"
	"github.com/akos011221/serverless/pkg/server"
	"github.com/akos011221/serverless/pkg/storage"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "serverless",
	Short: "Serverless platform",
	Long:  "Serverless platform that allows you to execute your functions in isolated Docker containers with HTTP triggers.",
}

// config holds global CLI flags, so the platform is configurable without code change.
var config struct {
	configFile string
}

// init configures CLI flags, binding them to the config struct.
func init() {
	rootCmd.PersistentFlags().StringVar(&config.configFile, "config", "config/config.yaml",
		"Path to the YAML configuration file")
}

// runServer starts the server component of the platform.
func runServer(cmd *cobra.Command, args []string) {
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})
	log.SetLevel(logrus.InfoLevel)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// SQLite storage for function metadata
	store, err := storage.NewStore("serverless.db", log)
	if err != nil {
		log.WithError(err).Fatal("failed to initialize storage")
	}

	// Server that handles the function deployment and invocation
	srv, err := server.NewServer(store, log)
	if err != nil {
		log.WithError(err).Fatal("Failed to initialize server")
	}

	// Handle shutdown signals (Ctrl+C, SIGTERM) for graceful termination
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Run the server in goroutine to allow signal handling in the
	// main thread
	go func() {
		if err := srv.Run(ctx); err != nil {
			log.WithError(err).Fatal("Server stopped unexpectedly")
		}
	}()

	// Wait for a shutdown signal
	<-sigChan
	log.Info("Received shutdown signal, stopping...")

	// Cancel the context to trigger shutdown of all components.
	cancel()

	log.Info("Platform stopped")

}

func main() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "run",
		Short: "Start the serverless platform server",
		Run:   runServer,
	})

	log := logrus.New()
	log.SetFormatter(&logrus.TextFormatter{ForceColors: true})
	log.SetLevel(logrus.InfoLevel)
	cli.RegisterCommands(rootCmd, config.configFile, log)

	if err := rootCmd.Execute(); err != nil {
		log.WithError(err).Fatal("CLI execution failed")
		os.Exit(1)
	}
}
