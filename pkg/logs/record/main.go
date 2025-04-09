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
var typ = os.Getenv("TYPE")
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
				creationTime := pod.CreationTimestamp.Time
				go func(podName, podNs string) {
					time.Sleep(time.Second * 5) // Wait for the pod to be ready
					logToDebug("New pod added: " + event.Object.(*v1.Pod).Name)
					dir := filepath.Join(pkg.ConfigData.LogsPath, namespace)
					if err := os.MkdirAll(dir, 0755); err != nil {
						logToDebug(err.Error())
						return
					}
					path := filepath.Join(dir, podName+".log")
					file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
					if err != nil {
						logToDebug(err.Error())
					}
					defer file.Close() // Remember to close the file
					podNameToLogPath[podName] = path
					logToDebug("TYPE IS " + typ)
					// Check whether pod's parent matched the deployment
					owner := pod.GetOwnerReferences()

					// It's a standalone pod
					if len(owner) == 0 {
						logToDebug("HERE")
						logToDebug("No owner references found for pod: " + podName)
					} else {
						logToDebug("NOWHERE")
						ref := owner[0]
						if ref.Kind == "ReplicaSet" {
							rsName := ref.Name
							ns := podNs

							rs, err := cs.AppsV1().ReplicaSets(ns).Get(ctx, rsName, metav1.GetOptions{})
							if err != nil {
								logToDebug(fmt.Sprintf("Error fetching ReplicaSet %s: %v", rsName, err))
							}
							//Its a standalone replicaset
							if len(rs.OwnerReferences) == 0 {
								logToDebug("No owner references found for rs: " + podName)
								path = filepath.Join(pkg.ConfigData.LogsPath, namespace, fmt.Sprintf("replicaset.%s.metadata", rsName))
								f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
								if err != nil {
									logToDebug(err.Error())
								}
								defer f.Close() // Remember to close the file
								f.WriteString(fmt.Sprintf("%s ; %s\n", creationTime.Format("2006-01-02 15:04:05"), podName))
							} else {
								rsOwnerRef := rs.OwnerReferences[0]
								if rsOwnerRef.Kind == "Deployment" {
									deploymentName := rsOwnerRef.Name
									deployment, err := cs.AppsV1().Deployments(ns).Get(ctx, deploymentName, metav1.GetOptions{})
									if err != nil {
										logToDebug(fmt.Sprintf("Error fetching Deployment %s: %v", deploymentName, err))
									}
									//Its a standalone deployment
									if len(deployment.OwnerReferences) == 0 {
										logToDebug("Parent deployment found for pod: " + podName)
										path = filepath.Join(pkg.ConfigData.LogsPath, namespace, fmt.Sprintf("deployment.%s.metadata", deploymentName))
										f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
										if err != nil {
											logToDebug(err.Error())
										}
										defer f.Close() // Remember to close the file
										f.WriteString(fmt.Sprintf("%s ; %s\n", creationTime.Format("2006-01-02 15:04:05"), podName))
									}
								}
							}
						}
					}

					logToDebug("Watching logs for pod: " + podName)
					for {
						opts := &v1.PodLogOptions{
							Follow: true,
						}
						req := cs.CoreV1().Pods(namespace).GetLogs(podName, opts)
						stream, err := req.Stream(ctx)
						if err != nil {
							logToDebug(err.Error())
							return
						}
						defer stream.Close()
						if _, err := io.Copy(file, stream); err != nil {
							logToDebug(err.Error())
						}
						logToDebug("Stream closed for pod: " + podName)
						break
					}

				}(pod.Name, pod.Namespace)
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
