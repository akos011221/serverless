package main

import (
	"context"
	"net/http"
	"os"

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

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	http.ListenAndServe(":1234", nil)
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

	if err := rootCmd.Execute(); err != nil {
		log.WithError(err).Fatal("CLI execution failed")
		os.Exit(1)
	}
}
