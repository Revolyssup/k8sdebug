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
			cmd.Println("Total pods scanned: ", readAllPodLogs(podNames, logPaths, timeStamps, maxLinesToRead))
		},
	}
	cmd.Flags().BoolVar(&onlyName, "only-names", false, "display only names")
	return cmd
}

func readAllPodLogs(podNames []string, logPath []string, timestamps []string, maxLines int) int {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.Debug)
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
	fmt.Println("INITIAL", initial, "FINAL", final)
	for i := initial; i < final; i++ {
		if onlyName {
			if i == initial {
				fmt.Fprintln(w, "Pod Name\tCreated At")
			}
			fmt.Fprintln(w, fmt.Sprintf("%s\t%s", podNames[i], timestamps[i]))
			if i == final-1 {
				w.Flush()
			}
			continue
		}
		file, err := os.Open(logPath[i])
		if err != nil {
			fmt.Println("file not found for pod:", podNames[i])
			continue
		}
		defer file.Close()
		logs := readNLines(file, maxLines)
		fmt.Println(pkg.ColorLine(fmt.Sprintf("Logs from pod: %s - %s", podNames[i], timestamps[i]), pkg.ColorYellow), string(logs))
	}
	return len(podNames) - initial
}

func readNLines(file *os.File, n int) string {
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
	fmt.Println("Total lines ", len(lines), " and n is ", n)
	if bottomFile {
		initial = len(lines) - n
	} else {
		final = n
	}
	if initial < 0 {
		initial = 0
	}
	return strings.Join(lines[initial:final], "\n")
}
