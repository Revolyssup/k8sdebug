package pkg

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var ConfigFilePath string

type Color string

const (
	ColorRed    Color = "\033[31m"
	ColorGreen  Color = "\033[32m"
	ColorYellow Color = "\033[33m"
	ColorReset  Color = "\033[0m"
)

type Config struct {
	LogsPath  string
	LoggerPID int
}

var ConfigData Config = Config{
	LogsPath: "/tmp/k8sdebug/logs",
}

func ColorizeDiff(diff string) string {
	var b strings.Builder
	lines := strings.Split(diff, "\n")

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "-"):
			b.WriteString(string(ColorRed))
			b.WriteString(line)
			b.WriteString(string(ColorReset) + "\n")
		case strings.HasPrefix(line, "+"):
			b.WriteString(string(ColorGreen))
			b.WriteString(line)
			b.WriteString(string(ColorReset) + "\n")
		default:
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	return b.String()
}

func ColorLine(s string, color Color) string {
	var b strings.Builder
	lines := strings.Split(s, "\n")
	for _, line := range lines {
		b.WriteString(string(color))
		b.WriteString(line)
		b.WriteString(string(ColorReset) + "\n")

	}
	return b.String()
}

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Errorf("failed to get home directory: %w", err))
	}
	ConfigFilePath = filepath.Join(home, ".k8sdebug", ".env")
	if _, err := os.Stat(ConfigFilePath); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(ConfigFilePath), 0755); err != nil {
			panic(fmt.Errorf("failed to create config directory: %w", err))
		}
		if err := os.WriteFile(ConfigFilePath, []byte(""), 0644); err != nil {
			panic(fmt.Errorf("failed to create config file: %w", err))
		}
	}
	file, err := os.Open(ConfigFilePath)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		vars := strings.Split(line, "=")
		if len(vars) != 2 {
			continue
		}
		key := strings.TrimSpace(vars[0])
		value := strings.TrimSpace(vars[1])
		switch key {
		case "LOGS_PATH":
			ConfigData.LogsPath = value
		case "LOGGER_PID":
			pid, err := strconv.Atoi(value)
			if err != nil {
				panic(fmt.Errorf("failed to parse LOGGER_PID: %w", err))
			}
			ConfigData.LoggerPID = pid
		default:
			continue
		}
	}

	if err := os.MkdirAll(ConfigData.LogsPath, 0755); err != nil {
		panic(fmt.Errorf("failed to create directory: %w", err))
	}
}
