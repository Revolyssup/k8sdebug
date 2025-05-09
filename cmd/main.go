package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/revolyssup/k8sdebug/pkg"
	"github.com/revolyssup/k8sdebug/pkg/logs"
	"github.com/revolyssup/k8sdebug/pkg/portforward"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := cobra.Command{
		Use:   "k8sdebug",
		Short: "Debug application in Kubernetes",
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			content := ""
			content += pkg.LOGS_PATH + "=" + pkg.ConfigData.LogsPath + "\n"
			pid, err := strconv.Atoi(strconv.Itoa(pkg.ConfigData.LoggerPID))
			if err != nil {
				cmd.Println("Error converting LoggerPID to string:", err)
				return
			}
			content += fmt.Sprintf("%s=%d\n", pkg.LOGGER_PID, pid)
			if err := os.WriteFile(pkg.ConfigFilePath, []byte(content), 0644); err != nil {
				cmd.Println("Error writing config file:", err)
			}
		},
	}
	rootCmd.AddCommand(logs.NewCommand())
	rootCmd.AddCommand(portforward.NewCommand())
	rootCmd.Execute()
}
