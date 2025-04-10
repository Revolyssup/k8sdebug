package logs

import (
	"fmt"
	"path/filepath"

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
			if len(args) >= 1 && args[0] == "setpath" {
				if len(args) == 1 || args[1] == "" {
					cmd.Println(pkg.ColorLine("Please provide a valid path!", pkg.ColorRed))
					return
				}
				path := args[1]
				fp, err := filepath.Abs(path)
				if err != nil {
					cmd.Println(pkg.ColorLine("Please provide a valid path!", pkg.ColorRed))
				}
				pkg.ConfigData.LogsPath = fp
				return
			}
			if len(args) == 1 && args[0] == "getpath" {
				cmd.Println(pkg.ColorLine(fmt.Sprintf("LOGGER_PATH set to %s", pkg.ConfigData.LogsPath), pkg.ColorGreen))
			}
		},
	}
	cmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "default", "Name of the pod")
	cmd.PersistentFlags().StringVarP(&typ, "type", "t", "pod", "Name of the pod")
	cmd.PersistentFlags().IntVar(&maxPods, "max-pods", 10, "chronological index of the pod")
	cmd.PersistentFlags().BoolVar(&latestFirst, "latest", false, "reverse the order of the log files")
	cmd.PersistentFlags().BoolVarP(&bottomFile, "end-of-file", "e", false, "reverse the order of the logs")
	cmd.PersistentFlags().IntVar(&maxLinesToRead, "max-lines", 10, "maximum number of lines to read from the log file")
	cmd.PersistentFlags().IntVar(&tail, "tail", 10, "No. of lines to use for diff")
	cmd.AddCommand(newRecordCommand())
	cmd.AddCommand(newShowCommand())
	cmd.AddCommand(newCleanupCommand())
	cmd.AddCommand(newDiffCommand())
	return cmd
}
