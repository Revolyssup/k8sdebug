package logs

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/revolyssup/k8sdebug/pkg"
	"github.com/spf13/cobra"
)

var typ string

func newShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show logs of a pod",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]
			totalPods := 0
			switch typ {
			case "pod":
				path := filepath.Join(pkg.ConfigData.LogsPath, namespace, fmt.Sprintf("%s.log", name))
				logs, err := os.ReadFile(path)
				if err != nil {
					cmd.Println("No logs found for pod:", name)
					return
				}
				cmd.Println("-------------------------------------------")
				cmd.Println("Logs from pod ", name, ":", "\n", string(logs))

			case "deployment":
				path := filepath.Join(pkg.ConfigData.LogsPath, namespace, fmt.Sprintf("deployment.%s.metadata", name))
				f, err := os.Open(path)
				if err != nil {
					cmd.Println("No logs found for deployment:", name)
					return
				}
				buf := bufio.NewScanner(f)
				buf.Split(bufio.ScanLines)
				for buf.Scan() {
					line := buf.Text()
					if line == "" {
						continue
					}
					ele := strings.Split(line, ";")
					if len(ele) < 2 {
						continue
					}
					// timestamp := strings.TrimSpace(ele[0])
					podName := strings.TrimSpace(ele[1])
					logPath := filepath.Join(pkg.ConfigData.LogsPath, namespace, fmt.Sprintf("%s.log", podName))
					logs, err := os.ReadFile(logPath)
					if err != nil {
						cmd.Println("No logs found for pod:", name)
						return
					}
					cmd.Println("-------------------------------------------")
					cmd.Println("Logs from pod ", podName, ":", "\n", string(logs))
					totalPods++
				}

			case "replicaset":
				path := filepath.Join(pkg.ConfigData.LogsPath, namespace, fmt.Sprintf("replicaset.%s.metadata", name))
				f, err := os.Open(path)
				if err != nil {
					cmd.Println("No logs found for replicaset:", name)
					return
				}
				buf := bufio.NewScanner(f)
				buf.Split(bufio.ScanLines)
				for buf.Scan() {
					line := buf.Text()
					if line == "" {
						continue
					}
					ele := strings.Split(line, ";")
					if len(ele) < 2 {
						continue
					}
					// timestamp := strings.TrimSpace(ele[0])
					podName := strings.TrimSpace(ele[1])
					logPath := filepath.Join(pkg.ConfigData.LogsPath, namespace, fmt.Sprintf("%s.log", podName))
					logs, err := os.ReadFile(logPath)
					if err != nil {
						cmd.Println("No logs found for pod:", name)
						return
					}
					cmd.Println("-------------------------------------------")
					cmd.Println("Logs from pod ", podName, ":", "\n", string(logs))
					totalPods++
				}
			}
			cmd.Println("Total correlated pods found for", name, "=", totalPods)
		},
	}
	return cmd
}
