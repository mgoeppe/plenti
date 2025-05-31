package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DataPoint represents a single data point in the database
type DataPoint struct {
	ID        uint      `gorm:"primarykey"`
	CreatedAt time.Time `gorm:"index"`
	ModuleID  string    `gorm:"index"`
	FieldID   string    `gorm:"index"`
	Value     float64
	Unit      string
}

// Database connection
var db *gorm.DB

var saveCmd = &cobra.Command{
	Use:   "save",
	Short: "Save data from Plenticore to database",
	Long:  `Retrieve current data from the Plenticore inverter and save it to a database.`,
	PreRun: func(cmd *cobra.Command, args []string) {
		// Initialize the database connection
		dbPath := viper.GetString("database.path")
		if dbPath == "" {
			dbPath = "plenticore.db"
			logrus.Infof("No database path specified, using default: %s", dbPath)
		}

		// Configure GORM logger based on our app's log level
		gormLogLevel := logger.Silent
		if logrus.GetLevel() == logrus.DebugLevel {
			gormLogLevel = logger.Info
		} else if logrus.GetLevel() == logrus.InfoLevel {
			gormLogLevel = logger.Warn
		} else if logrus.GetLevel() == logrus.WarnLevel {
			gormLogLevel = logger.Error
		}

		// Custom GORM logger
		gormLogger := logger.New(
			logrus.StandardLogger(),
			logger.Config{
				SlowThreshold:             time.Second,
				LogLevel:                  gormLogLevel,
				IgnoreRecordNotFoundError: true,
				ParameterizedQueries:      false,
			},
		)

		var err error
		db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
			Logger: gormLogger,
		})
		if err != nil {
			logrus.Fatalf("Failed to connect to database: %v", err)
		}

		// Auto migrate the schema
		err = db.AutoMigrate(&DataPoint{})
		if err != nil {
			logrus.Fatalf("Failed to migrate database schema: %v", err)
		}
		logrus.Infof("Connected to database: %s", dbPath)
	},
	Run: func(cmd *cobra.Command, args []string) {
		// Check for interval flag
		intervalStr, _ := cmd.Flags().GetString("interval")

		// If interval is not set, just run once
		if intervalStr == "" {
			saveDataOnce()
			return
		}

		// Parse interval duration
		interval, err := time.ParseDuration(intervalStr)
		if err != nil {
			logrus.Fatalf("Invalid interval format: %v", err)
		}

		if interval < time.Second {
			logrus.Fatal("Interval must be at least 1 second")
		}

		logrus.Infof("Running in continuous mode with interval: %s", interval)
		logrus.Info("Press Ctrl+C to stop...")

		// Set up signal handling for graceful shutdown
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		// Ticker for periodic saves
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		// Save immediately on start
		go saveDataOnce()

		// Run until interrupted
		for {
			select {
			case <-ticker.C:
				go saveDataOnce()
			case <-sigChan:
				logrus.Info("Received shutdown signal, exiting...")
				return
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(saveCmd)

	// Add flags specific to the save command
	saveCmd.Flags().Bool("summary", false, "Print a summary after saving")
	saveCmd.Flags().StringP("database", "d", "plenticore.db", "Database file path")
	saveCmd.Flags().StringP("interval", "i", "", "Run continuously with the specified interval (e.g. '30s', '5m', '1h')")

	// Bind flags to viper config
	viper.BindPFlag("database.printSummary", saveCmd.Flags().Lookup("summary"))
	viper.BindPFlag("database.path", saveCmd.Flags().Lookup("database"))
	viper.BindPFlag("database.interval", saveCmd.Flags().Lookup("interval"))
}

// saveDataOnce retrieves data from Plenticore and saves it to the database
func saveDataOnce() {
	logrus.Debug("Retrieving data from Plenticore for database storage...")

	// Fetch the data
	data := commandCtx.Client.Data(ParseFields())

	// Calculate total data points for logging
	totalPoints := 0
	for _, module := range data {
		totalPoints += len(module.Fields)
	}
	logrus.Debugf("Retrieved %d data points from %d modules", totalPoints, len(data))

	// Start a database transaction
	tx := db.Begin()
	if tx.Error != nil {
		logrus.Errorf("Failed to begin transaction: %v", tx.Error)
		return
	}

	// Record the timestamp when the data was fetched
	timestamp := time.Now()
	points := 0

	// Store each data point
	for _, module := range data {
		for _, field := range module.Fields {
			dataPoint := DataPoint{
				CreatedAt: timestamp,
				ModuleID:  module.ModuleID,
				FieldID:   field.ID,
				Value:     field.Value,
				Unit:      field.Unit,
			}

			if result := tx.Create(&dataPoint); result.Error != nil {
				tx.Rollback()
				logrus.Errorf("Failed to save data point %s/%s: %v",
					module.ModuleID, field.ID, result.Error)
				return
			}
			points++
		}
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		logrus.Errorf("Failed to commit transaction: %v", err)
		return
	}

	logrus.Infof("Successfully saved %d data points to database", points)

	// Print a summary if requested
	if viper.GetBool("database.printSummary") {
		fmt.Printf("Saved %d data points from %d modules at %s\n",
			points, len(data), timestamp.Format(time.RFC3339))
	}
}
