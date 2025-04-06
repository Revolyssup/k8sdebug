package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/revolyssup/k8sdebug/pkg"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var namespace = os.Getenv("NAMESPACE")
var file *os.File

func logToDebug(msg string) {
	if _, err := file.WriteString(fmt.Sprintf("%s - %s\n", time.Now().Format("2006-01-02 15:04:05"), msg)); err != nil {
		fmt.Println(err)
	}
}

func main() {
	logToDebug("Starting logger...")
	if _, err := os.Stat(filepath.Join(pkg.ConfigData.LogsPath, ".k8s.debug")); os.IsNotExist(err) {
		file, err = os.Create(filepath.Join(pkg.ConfigData.LogsPath, ".k8s.debug"))
		if err != nil {
			panic(err)
		}
	} else {
		file, err = os.OpenFile(filepath.Join(pkg.ConfigData.LogsPath, ".k8s.debug"), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			panic(err)
		}
	}

	logToDebug("Recording logs  in ns..." + namespace)
	kubeconfig := clientcmd.NewDefaultClientConfigLoadingRules().GetDefaultFilename()
	logToDebug("Kubeconfig file:" + kubeconfig)
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		logToDebug(err.Error())
	}
	cs := kubernetes.NewForConfigOrDie(config)
	podNameToLogPath := make(map[string]string, 0)
	ctx, cancel := context.WithCancel(context.Background())

	sigchan := make(chan os.Signal)
	signal.Notify(sigchan, os.Interrupt)
	go func() {
		watcher, err := cs.CoreV1().Pods(namespace).Watch(context.TODO(), metav1.ListOptions{})
		if err != nil {
			logToDebug(err.Error())
		}
		logToDebug("Watching for new pods in namespace" + namespace)
		for event := range watcher.ResultChan() {
			switch event.Type {
			case watch.Added:
				pod := event.Object.(*v1.Pod)
				go func(name string) {
					logToDebug("New pod added: " + event.Object.(*v1.Pod).Name)
					pod := event.Object.(*v1.Pod)
					dir := filepath.Join(pkg.ConfigData.LogsPath, namespace)
					if err := os.MkdirAll(dir, 0755); err != nil {
						logToDebug(err.Error())
						return
					}
					path := filepath.Join(dir, pod.Name+".log")
					file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
					if err != nil {
						logToDebug(err.Error())
					}
					defer file.Close() // Remember to close the file
					podNameToLogPath[pod.Name] = path
					opts := &v1.PodLogOptions{
						Follow: true,
					}
					req := cs.CoreV1().Pods(namespace).GetLogs(name, opts)
					stream, err := req.Stream(ctx)
					if err != nil {
						logToDebug(err.Error())
						return
					}
					defer stream.Close()
					if _, err := io.Copy(file, stream); err != nil {
						logToDebug(err.Error())
					}
				}(pod.Name)
			}
		}
	}()
	logToDebug("Pods in namespace" + namespace)
	select {
	case <-sigchan: // Wait for interrupt
		cancel()
	}
	logToDebug("Stopping logger...")
}
