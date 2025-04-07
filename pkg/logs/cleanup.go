package logs

import (
	"os"
	"path/filepath"

	"github.com/revolyssup/k8sdebug/pkg"
	"github.com/spf13/cobra"
)

func newCleanupCommand() *cobra.Command {
	var hardClean bool
	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Cleanup logs of a pod",
		Run: func(cmd *cobra.Command, args []string) {
			// if pkg.ConfigData.LoggerPID != 0 {
			// 	cmd.Println("Logger still running. Please first stop the logger")
			// 	return
			// }
			cmd.Println("Cleaning up logs...")
			if hardClean {
				if err := os.RemoveAll(pkg.ConfigData.LogsPath); err != nil {
					cmd.Println("Error cleaning up logs:", err)
					return
				}
			} else {
				if err := os.RemoveAll(filepath.Join(pkg.ConfigData.LogsPath, namespace)); err != nil {
					cmd.Println("Error cleaning up logs:", err)
					return
				}
			}
			cmd.Println("Logs cleaned up successfully.")
		},
	}

	cmd.Flags().BoolVar(&hardClean, "hard", false, "Whether to hard clean the logs and delete everything")
	return cmd
}
