package cmd

import (
	"sort"
	"strings"

	"github.com/matoubidou/plenticore-to-db/pkg/plenticore"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var dataCmd = &cobra.Command{
	Use:   "data",
	Short: "Get data from Plenticore inverter",
	Long:  `Retrieve current data from the Plenticore inverter and optionally save it to a database.`,
	Run: func(cmd *cobra.Command, args []string) {
		data := commandCtx.Client.Data(ParseFields())
		for _, module := range data {
			for _, field := range module.Fields {
				cmd.Printf("%s/%s: %f %s\n", module.ModuleID, field.ID, field.Value, field.Unit)
			}
		}
	},
}

func ParseFields() []plenticore.Fields {
	m2f := make(map[string][]string)
	for _, field := range viper.GetStringSlice("plenticore.fields") {
		p := strings.SplitN(field, "/", 2)
		if len(p) != 2 {
			logrus.Fatal("Invalid field format: %s. Expected format is 'moduleID/fieldID'\n", field)
		}
		moduleID, fieldID := p[0], p[1]
		m2f[moduleID] = append(m2f[moduleID], fieldID)
	}
	fields := make([]plenticore.Fields, 0, len(m2f))
	for moduleID, fieldIDs := range m2f {
		fields = append(fields, plenticore.Fields{
			ModuleID: moduleID,
			FieldIDs: fieldIDs,
		})
	}
	if len(fields) == 0 {
		fields = commandCtx.Client.Fields()
	}

	sort.Slice(fields, func(i, j int) bool {
		return fields[i].ModuleID < fields[j].ModuleID
	})
	for i := range fields {
		sort.Strings(fields[i].FieldIDs)
	}
	return fields
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
