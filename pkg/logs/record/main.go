package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/revolyssup/k8sdebug/pkg"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var namespace = os.Getenv("NAMESPACE")
var typ = os.Getenv("TYPE")
var debugFile *os.File

func logToDebug(msg string) {
	if _, err := debugFile.WriteString(fmt.Sprintf("%s - %s\n", time.Now().Format("2006-01-02 15:04:05"), msg)); err != nil {
		fmt.Println(err)
	}
}

func main() {
	logToDebug("Starting logger...")
	//Create the debug file
	if _, err := os.Stat(filepath.Join(pkg.ConfigData.LogsPath, ".k8s.debug")); os.IsNotExist(err) {
		debugFile, err = os.Create(filepath.Join(pkg.ConfigData.LogsPath, ".k8s.debug"))
		if err != nil {
			panic(err)
		}
	} else {
		debugFile, err = os.OpenFile(filepath.Join(pkg.ConfigData.LogsPath, ".k8s.debug"), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			panic(err)
		}
	}

	kubeconfig := clientcmd.NewDefaultClientConfigLoadingRules().GetDefaultFilename()
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		logToDebug(err.Error())
	}
	cs := kubernetes.NewForConfigOrDie(config)
	ctx, cancel := context.WithCancel(context.Background())

	/*
		Since the pods returned are not chronologically order. First we list the existing pods and sort them.
		After that resource Version, we create a watch and assume that the pods come in chronological order.
		This order is important because the replicaset.metadata and deployment.metadata are Append only logs containing in
		chronological order.
	*/
	initialList, err := cs.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logToDebug(err.Error())
	}
	pods := mergeSort(initialList.Items)
	for _, pod := range pods {
		processPod(ctx, cs, &pod, namespace)
	}

	watcher, err := cs.CoreV1().Pods(namespace).Watch(context.TODO(), metav1.ListOptions{
		ResourceVersion: initialList.ResourceVersion,
	})
	if err != nil {
		logToDebug(err.Error())
	}
	logToDebug("Watching for new pods in namespace" + namespace)
	go func() {
		for event := range watcher.ResultChan() {
			switch event.Type {
			case watch.Added:
				pod := event.Object.(*v1.Pod)
				go processPod(ctx, cs, pod, namespace)
			}
		}
	}()

	var wg sync.WaitGroup
	sigchan := make(chan os.Signal)
	signal.Notify(sigchan, os.Interrupt)
	wg.Add(1)
	go func() {
		select {
		case <-sigchan: // Wait for interrupt
			cancel()
			wg.Done()
		}
	}()
	wg.Wait()
	logToDebug("Stopping logger...")
}

// Process pod first synchronously append metadata to the metadata log because order is important.
// And then starts a go routine that watched for pods and writes to log files.
func processPod(ctx context.Context, cs *kubernetes.Clientset, pod *v1.Pod, namespace string) {
	creationTime := pod.CreationTimestamp.Time
	podName := pod.Name
	// podNs := pod.Namespace
	// TODO: Fix this 5 second wait
	time.Sleep(time.Second * 5) // Wait for the pod to be ready
	logToDebug("New pod added: " + pod.Name + "on time " + creationTime.Format("2006-01-02 15:04:05"))
	dir := filepath.Join(pkg.ConfigData.LogsPath, namespace)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logToDebug(err.Error())
		return
	}
	owner := GetLastNode(&PodNode{
		pod: pod,
		cs:  cs,
	})
	path := filepath.Join(pkg.ConfigData.LogsPath, namespace, fmt.Sprintf("%s.%s.metadata", strings.ToLower(owner.Type()), owner.Name()))

	// Use the advisory lock to write in metadata file
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		logToDebug(err.Error())
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		fmt.Println("Error locking file:", err)
		return
	}
	f.WriteString(fmt.Sprintf("%s ; %s\n", creationTime.Format("2006-01-02 15:04:05"), podName))
	syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	f.Close()

	//Start watching and recording logs
	go func(podName string) {
		logToDebug("Watching logs for pod: " + podName)
		path = filepath.Join(dir, podName+".log")
		file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			logToDebug(err.Error())
		}
		defer file.Close() // Remember to close the file
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
	}(podName)

}

func mergeSort(pods []v1.Pod) []v1.Pod {
	if len(pods) <= 1 {
		sortedPods := pods
		return sortedPods
	}
	mid := len(pods) / 2
	return mergeSorted(mergeSort(pods[:mid]), mergeSort(pods[mid:]))
}

func mergeSorted(leftSort []v1.Pod, rightSorted []v1.Pod) []v1.Pod {
	newList := []v1.Pod{}
	leftPtr := 0
	rightPtr := 0
	for leftPtr < len(leftSort) && rightPtr < len(rightSorted) {
		if isSmaller(leftSort[leftPtr], rightSorted[rightPtr]) {
			newList = append(newList, leftSort[leftPtr])
			leftPtr++
		} else {
			newList = append(newList, rightSorted[rightPtr])
			rightPtr++
		}
	}

	if leftPtr < len(leftSort) {
		newList = append(newList, leftSort[leftPtr:]...)
	}
	if rightPtr < len(rightSorted) {
		newList = append(newList, rightSorted[rightPtr:]...)
	}
	return newList

}

func isSmaller(a, b v1.Pod) bool {
	return b.CreationTimestamp.After(a.CreationTimestamp.Time)
}

type Node interface {
	Next() Node
	Type() string
	Name() string
}

func GetLastNode(n Node) Node {
	for {
		logToDebug("node process" + n.Type())
		nextNode := n.Next()
		if nextNode == nil {
			return n
		}
		logToDebug("got nextnode" + nextNode.Type())
		n = nextNode
	}
}

type PodNode struct {
	pod *v1.Pod
	cs  *kubernetes.Clientset
}
type ReplicasetNode struct {
	rs *appsv1.ReplicaSet
	cs *kubernetes.Clientset
}
type DeploymentNode struct {
	ds *appsv1.Deployment
	cs *kubernetes.Clientset
}

func (p *PodNode) Next() Node {
	owners := p.pod.GetOwnerReferences()

	if len(owners) == 0 {
		return nil
	}

	owner := owners[0]
	name := owner.Name
	switch owner.Kind {
	case "ReplicaSet":
		rs, err := p.cs.AppsV1().ReplicaSets(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			logToDebug(fmt.Sprintf("Error fetching ReplicaSet %s: %v", name, err))
			return nil
		}
		rsNode := &ReplicasetNode{
			rs: rs,
			cs: p.cs,
		}
		return rsNode
	default:
		return nil
	}
}

func (p *PodNode) Type() string {
	return "Pod"
}

func (p *PodNode) Name() string {
	return p.pod.Name
}

func (p *ReplicasetNode) Next() Node {
	owners := p.rs.GetOwnerReferences()
	if len(owners) == 0 {
		return nil
	}

	owner := owners[0]
	name := owner.Name
	switch owner.Kind {
	case "Deployment":
		deps, err := p.cs.AppsV1().Deployments(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			logToDebug(fmt.Sprintf("Error fetching ReplicaSet %s: %v", name, err))
			return nil
		}
		depsNode := DeploymentNode{
			ds: deps,
			cs: p.cs,
		}
		return &depsNode
	default:
		return nil
	}
}

func (rs *ReplicasetNode) Name() string {
	return rs.rs.Name
}
func (p *ReplicasetNode) Type() string {
	return "ReplicaSet"
}

func (p *DeploymentNode) Next() Node {
	return nil
}

func (p *DeploymentNode) Type() string {
	return "Deployment"
}
func (dp *DeploymentNode) Name() string {
	return dp.ds.Name
}
