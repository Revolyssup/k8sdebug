package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/revolyssup/k8sdebug/pkg"
	"github.com/revolyssup/k8sdebug/pkg/logs"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := cobra.Command{
		Use:   "k8sdebug",
		Short: "Debug application in Kubernetes",
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			cmd.Println("Post run command executed")
			content := ""
			content += "LOGS_PATH=" + pkg.ConfigData.LogsPath + "\n"
			fmt.Println("LoggerPID:", pkg.ConfigData.LoggerPID)
			pid, err := strconv.Atoi(strconv.Itoa(pkg.ConfigData.LoggerPID))
			if err != nil {
				cmd.Println("Error converting LoggerPID to string:", err)
				return
			}
			content += fmt.Sprintf("LOGGER_PID=%d\n", pid)
			if err := os.WriteFile(pkg.ConfigFilePath, []byte(content), 0644); err != nil {
				cmd.Println("Error writing config file:", err)
			}
		},
	}
	rootCmd.AddCommand(logs.NewCommand())
	rootCmd.Execute()
}
