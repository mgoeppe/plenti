package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var fieldsCmd = &cobra.Command{
	Use:   "fields",
	Short: "List fields from Plenticore",
	Long:  `List all fields provided by the Plenticore inverter API. These fields can be used to configure filtering in your queries.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Client is already initialized in the root command's PersistentPreRun

		modules := commandCtx.Client.Fields()
		for _, module := range modules {
			for _, fieldID := range module.FieldIDs {
				cmd.Printf("- %s/%s\n", module.ModuleID, fieldID)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(fieldsCmd)

	// Local flags
	fieldsCmd.Flags().BoolP("save", "d", false, "Save field list to database")
	_ = viper.BindPFlag("saveFieldsToDb", fieldsCmd.Flags().Lookup("save"))
}
