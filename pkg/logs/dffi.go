package logs

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
	"github.com/revolyssup/k8sdebug/pkg"
	"github.com/spf13/cobra"
)

// Example color definitions
const (
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorReset  = "\033[0m"
)

func colorizeDiff(diff string) string {
	var b strings.Builder
	lines := strings.Split(diff, "\n")

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "-"):
			b.WriteString(ColorRed) // Red
			b.WriteString(line)
			b.WriteString(ColorReset + "\n")
		case strings.HasPrefix(line, "+"):
			b.WriteString(ColorGreen) // Green
			b.WriteString(line)
			b.WriteString(ColorReset + "\n")
		default:
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	return b.String()
}
func newDiffCommand() *cobra.Command {
	var tail int
	var excludePatterns []string
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Show diff of pods",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]
			podSlice := make([]string, 0)
			logSlice := make([]string, 0)
			switch typ {
			case "pod":
				// path := filepath.Join(pkg.ConfigData.LogsPath, namespace, fmt.Sprintf("%s.log", name))
				// logs, err := os.ReadFile(path)
				// if err != nil {
				// 	cmd.Println("No logs found for pod:", name)
				// 	return
				// }
				// cmd.Println("-------------------------------------------")
				// cmd.Println("Logs from pod ", name, ":", "\n", string(logs))
				cmd.Println("Diff are not available for pod:", name)

			case "deployment":
				parsedLines := 0
				path := filepath.Join(pkg.ConfigData.LogsPath, namespace, fmt.Sprintf("deployment.%s.metadata", name))
				f, err := os.Open(path)
				if err != nil {
					cmd.Println("No logs found for deployment:", name)
					return
				}
				buf := bufio.NewScanner(f)
				buf.Split(bufio.ScanLines)
				for buf.Scan() && parsedLines < tail {
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
					podSlice = append(podSlice, podName)
					logSlice = append(logSlice, string(logs))
					parsedLines++
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
				parsedLines := 0
				for buf.Scan() && parsedLines < tail {
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
					podSlice = append(podSlice, podName)
					logSlice = append(logSlice, string(logs))
					parsedLines++
				}
			}
			if len(podSlice) == 0 {
				cmd.Println("No logs found for:", name)
				return
			}
			if len(podSlice) == 1 {
				cmd.Println("No diff available for single pod:", name)
				return
			}
			cmd.Println("Diff logs for pods:")
			for i := 0; i < len(podSlice)-1; i++ {
				j := i + 1
				cmd.Println("Diff between ", podSlice[i], " and ", podSlice[j], ":")
				diff := difflib.UnifiedDiff{
					A:        difflib.SplitLines(logSlice[i]),
					B:        difflib.SplitLines(logSlice[j]),
					FromFile: podSlice[i],
					ToFile:   podSlice[j],
					Context:  3,
				}
				result, err := difflib.GetUnifiedDiffString(diff)
				if err != nil {
					cmd.Println("Error getting diff:", err)
					return
				}
				if result == "" {
					cmd.Println("No diff found between ", podSlice[i], " and ", podSlice[j])
					continue
				}
				cmd.Println(colorizeDiff(result), "\n--------------------------------------------------")
			}
			cmd.Println("End of diff logs")
			cmd.Println(`
Total pods: `, len(podSlice))
		},
	}
	cmd.Flags().IntVar(&tail, "tail", 10, "No. of lines to use for diff")

	// In your command flags:
	cmd.Flags().StringSliceVar(&excludePatterns, "exclude-patterns", nil, "Regex patterns to exclude from diff (e.g., timestamps)")
	return cmd
}

func preprocessLines(lines []string, patterns []string) []string {
	processed := make([]string, len(lines))
	for i, line := range lines {
		for _, pattern := range patterns {
			re := regexp.MustCompile(pattern)
			line = re.ReplaceAllString(line, "")
		}
		processed[i] = strings.TrimSpace(line)
	}
	return processed
}
