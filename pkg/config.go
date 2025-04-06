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

type Config struct {
	LogsPath  string
	LoggerPID int
}

var ConfigData Config = Config{
	LogsPath: "/tmp/k8sdebug/logs",
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
