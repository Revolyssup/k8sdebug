package logs

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/revolyssup/k8sdebug/pkg"
	"github.com/spf13/cobra"
)

var typ string
var onlyName bool

func newShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show logs of a pod",
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
			podNames, logSlice, timeStamps := getPodLogs(podNames, logPaths, timeStamps)
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.Debug)
			for i := range podNames {
				if onlyName {
					if i == 0 {
						fmt.Fprintln(w, "Pod Name\tCreated At")
					}
					fmt.Fprintln(w, fmt.Sprintf("%s\t%s", podNames[i], timeStamps[i]))
					if i == len(podNames)-1 {
						w.Flush()
					}
				} else {
					fmt.Println(pkg.ColorLine(fmt.Sprintf("Logs from pod: %s - %s", podNames[i], timeStamps[i]), pkg.ColorYellow), string(logSlice[i]))
				}
			}
			cmd.Println("Total pods scanned: ", len(podNames))
		},
	}
	cmd.Flags().BoolVar(&onlyName, "only-names", false, "display only names")
	return cmd
}

func getPodLogs(podNames []string, logPath []string, timestamps []string) (filteredPodNames []string, logSlice []string, newTimestamps []string) {
	initial := 0
	final := len(podNames)
	if latestFirst {
		initial = len(podNames) - maxPods
	} else {
		final = maxPods
	}
	if initial < 0 {
		initial = 0
	}
	if final > len(podNames) {
		final = len(podNames)
	}
	for i := initial; i < final; i++ {
		filteredPodNames = append(filteredPodNames, podNames[i])
		newTimestamps = append(newTimestamps, timestamps[i])
		if onlyName {
			continue
		}
		file, err := os.Open(logPath[i])
		if err != nil {
			fmt.Println("file not found for pod:", podNames[i])
			continue
		}
		defer file.Close()
		logs := readNLines(file)
		logSlice = append(logSlice, logs)
	}
	return
}

func readNLines(file *os.File) string {
	n := maxLinesToRead
	var lines []string
	i := 0
	buf := bufio.NewScanner(file)
	initial := i
	for ; buf.Scan(); i++ {
		line := buf.Text()
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return ""
	}
	final := len(lines)
	if bottomFile {
		initial = len(lines) - n
	} else {
		final = n
	}
	if initial < 0 {
		initial = 0
	}
	if final >= len(lines) {
		final = len(lines) - 1
	}
	return strings.Join(lines[initial:final], "\n")
}
