package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/matoubidou/plenti/pkg/plenticore"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// CommandContext holds shared objects for all commands
type CommandContext struct {
	Client *plenticore.Client
}

var commandCtx CommandContext

var rootCmd = &cobra.Command{
	Use:   "plenti",
	Short: "A tool to interact with Plenticore API",
	Long: `A tool for authenticating with and retrieving data from
the Plenticore inverter API, and optionally storing the data in a database.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Skip client initialization for help and completion commands
		if cmd.Name() == "help" || cmd.Name() == "completion" {
			return
		}

		// Get configuration values
		server := viper.GetString("plenticore.server")
		password := viper.GetString("plenticore.password")

		if server == "" || password == "" {
			logrus.Fatal("Server address and password must be provided in config file, environment variables, or flags")
		}

		logrus.Infof("Connecting to Plenticore server: %s", server)
		commandCtx.Client = plenticore.NewClientOrDie(server, password)
		logrus.Debug("Client initialized successfully")
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		// Skip client cleanup for help and completion commands
		if cmd.Name() == "help" || cmd.Name() == "completion" || commandCtx.Client == nil {
			return
		}

		logrus.Debug("Closing Plenticore client connection")
		commandCtx.Client.Close()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringP("server", "s", "", "Plenticore server address")
	rootCmd.PersistentFlags().StringP("password", "p", "", "Plenticore password")
	rootCmd.PersistentFlags().String("log-level", "info", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringP("config-path", "c", ".", "Path to directory containing plenti.yaml config file")

	// Bind flags to viper config
	viper.BindPFlag("plenticore.server", rootCmd.PersistentFlags().Lookup("server"))
	viper.BindPFlag("plenticore.password", rootCmd.PersistentFlags().Lookup("password"))
	viper.BindPFlag("logLevel", rootCmd.PersistentFlags().Lookup("log-level"))
	viper.BindPFlag("configPath", rootCmd.PersistentFlags().Lookup("config-path"))
}

func initConfig() {
	// Set default configuration path and name
	viper.SetConfigName("plenti")
	viper.SetConfigType("yaml")

	// Add the config path from flag or use default
	configPath := viper.GetString("configPath")
	if configPath == "" {
		configPath = "."
	}
	viper.AddConfigPath(configPath)

	// Set default values
	viper.SetDefault("logLevel", "info")
	viper.SetDefault("configPath", ".")

	// Read environment variables with prefix PLENTICORE_
	viper.SetEnvPrefix("PLENTICORE")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Read the configuration file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			logrus.Warnf("Config file not found: %v", err)
		} else {
			logrus.Fatalf("Error reading config file: %v", err)
		}
	}

	// Set up logging level
	level, err := logrus.ParseLevel(viper.GetString("logLevel"))
	if err != nil {
		logrus.Warnf("Invalid log level: %s, using 'info'", viper.GetString("logLevel"))
		logrus.SetLevel(logrus.InfoLevel)
	} else {
		logrus.SetLevel(level)
	}

	logrus.Debugf("Using config file: %s", viper.ConfigFileUsed())
}
