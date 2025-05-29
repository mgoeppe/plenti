package cmd

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var dataCmd = &cobra.Command{
	Use:   "fields",
	Short: "List fields from Plenticore",
	Long:  `List all fields provided by the Plenticore inverter API. These fields can be used to configure filtering in your queries.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Client is already initialized in the root command's PersistentPreRun

		modules := commandCtx.Client.ProcessData()
		logrus.Info("Available fields from Plenticore:")
		for _, module := range modules {
			for _, fieldID := range module.FieldIDs {
				fmt.Printf("- %s/%s\n", module.ID, fieldID)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(dataCmd)

	// Local flags
	dataCmd.Flags().BoolP("save", "d", false, "Save data to database")
	viper.BindPFlag("saveToDb", dataCmd.Flags().Lookup("save"))
}
