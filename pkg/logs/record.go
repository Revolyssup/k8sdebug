package logs

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/revolyssup/k8sdebug/pkg"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

var (
	namespace string
)

func newRecordCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "record",
		Short: "Record logs of a pod",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println("Record command executed")
			switch args[0] {
			case "run":
				startLogger()
			case "stop":
				stopLogger()
			case "restart":
				stopLogger()
				startLogger()
			default:
				cmd.Println("Invalid argument. Use 'run' or 'stop'.")
			}
		},
	}
	return cmd
}

var binpath string

func startLogger() {
	if pkg.ConfigData.LoggerPID != 0 {
		//Find the process
		p, err := os.FindProcess(pkg.ConfigData.LoggerPID)
		//On Unix systems, FindProcess always succeeds and returns a Process for the given pid, regardless of whether the process exists.
		//To test whether the process actually exists, see whether p.Signal(syscall.Signal(0)) reports an error.
		if p == nil {
			panic(err)
		}
		if err := p.Signal(syscall.Signal(0)); err == nil {
			fmt.Println("Logger already running with PID:", pkg.ConfigData.LoggerPID)
			return
		}
	}
	// RUN will triggere old data to be gone
	//TODO: Is this really necessary?
	if err := os.RemoveAll(pkg.ConfigData.LogsPath); err != nil {
		fmt.Println("Error cleaning up logs:", err)
		return
	}
	fmt.Println("CLEANED UP")
	if _, err := os.Stat(binpath); os.IsNotExist(err) {
		// Does not exist, will try to build it
		fmt.Println("Building the logger...")
		cmdBuild := exec.Command("go", "build", "-o", binpath, "./pkg/logs/record/main.go")
		if err := cmdBuild.Run(); err != nil {
			klog.Fatalf("Failed to build logger: %v", err)
		}
	} else {
		fmt.Println("Logger already present.")
	}
	cmd := exec.Command(binpath)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("NAMESPACE=%s", namespace))
	cmd.Env = append(cmd.Env, fmt.Sprintf("LOGS_PATH=%s", pkg.ConfigData.LogsPath))
	cmd.Env = append(cmd.Env, fmt.Sprintf("TYPE=%s", typ))
	if err := cmd.Start(); err != nil {
		fmt.Println("Error starting logger:", err)
		return
	}
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	// Store PID
	pkg.ConfigData.LoggerPID = cmd.Process.Pid
	fmt.Println("Logger started with PID:", pkg.ConfigData.LoggerPID)
}

func stopLogger() {
	if pkg.ConfigData.LoggerPID == 0 {
		fmt.Println("Logger not running.")
		return
	}
	process, err := os.FindProcess(pkg.ConfigData.LoggerPID)
	if err != nil {
		fmt.Println("Error finding logger process:", err)
		return
	}
	if err := process.Signal(os.Interrupt); err != nil {
		fmt.Println("Error stopping logger:", err)
		return
	}
	fmt.Println("STOPPED")
}
func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Errorf("failed to get home directory: %w", err))
	}
	binpath = filepath.Join(home, ".k8sdebug", "bin")
}
