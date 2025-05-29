package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var dataCmd = &cobra.Command{
	Use:   "data",
	Short: "Get data from Plenticore inverter",
	Long:  `Retrieve current data from the Plenticore inverter and optionally save it to a database.`,
	Run: func(cmd *cobra.Command, args []string) {
		modules := commandCtx.Client.Fields()
		commandCtx.Client.Data(modules)
	},
}

func init() {
	rootCmd.AddCommand(dataCmd)

	// Local flags
	dataCmd.Flags().BoolP("save", "d", false, "Save data to database")
	dataCmd.Flags().StringP("output", "o", "", "Output file path for JSON data")

	// Bind flags to viper config
	viper.BindPFlag("saveToDb", dataCmd.Flags().Lookup("save"))
	viper.BindPFlag("outputFile", dataCmd.Flags().Lookup("output"))
}
