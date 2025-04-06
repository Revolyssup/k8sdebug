package logs

import (
	"github.com/revolyssup/k8sdebug/pkg"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Get logs of a pod",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println("Logs will be recorded in", pkg.ConfigData.LogsPath)
		},
	}
	cmd.AddCommand(newRecordCommand())
	return cmd
}
