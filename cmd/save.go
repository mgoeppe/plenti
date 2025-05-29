package cmd

import (
	"fmt"
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
		logrus.Info("Retrieving data from Plenticore for database storage...")

		// Fetch the data
		data := commandCtx.Client.Data(ParseFields())

		// Calculate total data points for logging
		totalPoints := 0
		for _, module := range data {
			totalPoints += len(module.Fields)
		}
		logrus.Infof("Retrieved %d data points from %d modules", totalPoints, len(data))

		// Start a database transaction
		tx := db.Begin()
		if tx.Error != nil {
			logrus.Fatalf("Failed to begin transaction: %v", tx.Error)
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
					logrus.Fatalf("Failed to save data point %s/%s: %v",
						module.ModuleID, field.ID, result.Error)
				}
				points++
			}
		}

		// Commit the transaction
		if err := tx.Commit().Error; err != nil {
			logrus.Fatalf("Failed to commit transaction: %v", err)
		}

		logrus.Infof("Successfully saved %d data points to database", points)

		// Print a summary if requested
		if viper.GetBool("database.printSummary") {
			fmt.Printf("Saved %d data points from %d modules at %s\n",
				points, len(data), timestamp.Format(time.RFC3339))
		}
	},
}

func init() {
	rootCmd.AddCommand(saveCmd)

	// Add flags specific to the save command
	saveCmd.Flags().Bool("summary", false, "Print a summary after saving")
	saveCmd.Flags().StringP("database", "d", "plenticore.db", "Database file path")

	// Bind flags to viper config
	viper.BindPFlag("database.printSummary", saveCmd.Flags().Lookup("summary"))
	viper.BindPFlag("database.path", saveCmd.Flags().Lookup("database"))
}
