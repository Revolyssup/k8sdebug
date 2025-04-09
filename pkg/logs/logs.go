package logs

import (
	"github.com/revolyssup/k8sdebug/pkg"
	"github.com/spf13/cobra"
)

var maxPods int
var latestFirst bool
var maxLinesToRead int
var bottomFile bool
var tail int

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Get logs of a pod",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println("Logs will be recorded in", pkg.ConfigData.LogsPath)
		},
	}
	cmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "default", "Name of the pod")
	cmd.PersistentFlags().StringVarP(&typ, "type", "t", "pod", "Name of the pod")
	cmd.PersistentFlags().IntVar(&maxPods, "max-pods", 10, "chronological index of the pod")
	cmd.PersistentFlags().BoolVarP(&latestFirst, "latest", "l", false, "reverse the order of the log files")
	cmd.PersistentFlags().BoolVarP(&bottomFile, "end-of-file", "e", false, "reverse the order of the logs")
	cmd.PersistentFlags().IntVar(&maxLinesToRead, "max-lines", 10, "maximum number of lines to read from the log file")
	cmd.PersistentFlags().IntVar(&tail, "tail", 10, "No. of lines to use for diff")
	cmd.AddCommand(newRecordCommand())
	cmd.AddCommand(newShowCommand())
	cmd.AddCommand(newCleanupCommand())
	cmd.AddCommand(newDiffCommand())
	return cmd
}
