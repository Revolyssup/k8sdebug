package logs

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
	"github.com/revolyssup/k8sdebug/pkg"
	"github.com/spf13/cobra"
)

func newDiffCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "diff logs of a pod",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]
			totalPods := 0
			podNames := make([]string, 0)
			logPaths := make([]string, 0)
			timeStamps := make([]string, 0)
			switch typ {
			case "pod":
				path := filepath.Join(pkg.ConfigData.LogsPath, namespace, fmt.Sprintf("%s.log", name))
				logs, err := os.ReadFile(path)
				if err != nil {
					cmd.Println("No logs found for pod:", name)
					return
				}
				cmd.Println(pkg.ColorLine("Logs from pod ", pkg.ColorYellow), name, ":", "\n", string(logs))

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
					if !latestFirst && totalPods >= maxPods {
						break
					}
					line := buf.Text()
					if line == "" {
						continue
					}
					ele := strings.Split(line, ";")
					if len(ele) < 2 {
						continue
					}
					podName := strings.TrimSpace(ele[1])
					logPath := filepath.Join(pkg.ConfigData.LogsPath, namespace, fmt.Sprintf("%s.log", podName))
					podNames = append(podNames, podName)
					logPaths = append(logPaths, logPath)
					timeStamps = append(timeStamps, ele[0])
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
					if !latestFirst && totalPods >= maxPods {
						break
					}
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
					podNames = append(podNames, podName)
					logPaths = append(logPaths, logPath)
					timeStamps = append(timeStamps, ele[0])
					totalPods++
				}
			}
			fmt.Println("HERE")
			printPodDiffs(getPodLogs(podNames, logPaths, timeStamps))
		},
	}
	cmd.Flags().BoolVar(&onlyName, "only-names", false, "display only names")
	return cmd
}

func printPodDiffs(podNames []string, logSlice []string, timestamps []string) {
	fmt.Println("HEREE", podNames)
	for i := 0; i < len(podNames)-1; i++ {
		j := i + 1
		fmt.Println("Diff between ", podNames[i], " and ", podNames[j], ":")
		diff := difflib.UnifiedDiff{
			A:        difflib.SplitLines(logSlice[i]),
			B:        difflib.SplitLines(logSlice[j]),
			FromFile: podNames[i],
			ToFile:   podNames[j],
			Context:  3,
		}
		result, err := difflib.GetUnifiedDiffString(diff)
		if err != nil {
			fmt.Println("Error getting diff:", err)
			return
		}
		if result == "" {
			fmt.Println("No diff found between ", podNames[i], " and ", podNames[j])
			continue
		}
		fmt.Println(pkg.ColorizeDiff(result), "\n--------------------------------------------------")
	}
}
