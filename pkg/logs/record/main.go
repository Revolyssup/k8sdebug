package main

import (
	"context"
	"encoding/json"
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
var labels = os.Getenv("LABELS")
var typ = os.Getenv("TYPE")
var debugFile *os.File

var indexFilePath = filepath.Join(pkg.ConfigData.LogsPath, "checkpoint.json")

type checkpoint struct {
	LastResourceVersion string
	FileOffsets         map[string]int64 // Filepath -> Offset to last entry in that file.
}

var checkpointData checkpoint

func initialiseDebugFile() {
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
	os.Stdout = debugFile
	os.Stderr = debugFile
}

func readCheckpoint() {
	// Open the file with read/write mode, create it if it doesn't exist
	file, err := os.OpenFile(indexFilePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		panic("failed to open/create checkpoint file: " + err.Error())
	}
	defer file.Close()

	// Read the file content
	bytCheckpnt, err := io.ReadAll(file)
	if err != nil {
		panic("failed to read checkpoint file: " + err.Error())
	}

	// If the file is empty (newly created), initialize default data
	if len(bytCheckpnt) == 0 {
		checkpointData = checkpoint{
			LastResourceVersion: "",
			FileOffsets:         make(map[string]int64),
		}
		// Marshal the default data and write it to the file
		data, err := json.Marshal(checkpointData)
		if err != nil {
			panic("failed to marshal default checkpoint data: " + err.Error())
		}
		if _, err := file.Write(data); err != nil {
			panic("failed to write default checkpoint data: " + err.Error())
		}
		return
	}

	// Unmarshal existing data
	if err := json.Unmarshal(bytCheckpnt, &checkpointData); err != nil {
		panic("failed to unmarshal checkpoint data: " + err.Error())
	}
}

func writeCheckpoint() {
	bytCheckpnt, err := json.Marshal(checkpointData)
	if err != nil {
		panic("could not read the checkpoint data at")
	}
	file, err := os.OpenFile(indexFilePath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	_, err = file.Write(bytCheckpnt)
	if err != nil {
		fmt.Println("Error writing to file:", err)
		return
	}
	file.Close()
}

func main() {
	fmt.Println("Starting logger...")
	initialiseDebugFile()
	readCheckpoint()
	defer writeCheckpoint()
	kubeconfig := clientcmd.NewDefaultClientConfigLoadingRules().GetDefaultFilename()
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		fmt.Println(err.Error())
	}
	cs := kubernetes.NewForConfigOrDie(config)
	ctx, cancel := context.WithCancel(context.Background())

	/*
		Since the pods returned are not chronologically order. First we list the existing pods and sort them.
		After that resource Version, we create a watch and assume that the pods come in chronological order.
		This order is important because the replicaset.metadata and deployment.metadata are Append only logs containing in
		chronological order.
	*/
	opts := metav1.ListOptions{}
	if labels != "" {
		opts.LabelSelector = labels
	}
	if checkpointData.LastResourceVersion != "" {
		opts.ResourceVersion = checkpointData.LastResourceVersion
	}
	initialList, err := cs.CoreV1().Pods(namespace).List(context.TODO(), opts)
	if err != nil {
		fmt.Println("Exiting runner..." + err.Error())
		return
	}
	pods := mergeSort(initialList.Items)
	for _, pod := range pods {
		processPod(ctx, cs, &pod, namespace)
	}
	opts.ResourceVersion = initialList.ResourceVersion
	watcher, err := cs.CoreV1().Pods(namespace).Watch(context.TODO(), opts)
	if err != nil {
		fmt.Println("Exiting runner..." + err.Error())
		return
	}
	fmt.Println("Watching for new pods in namespace" + namespace)
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
		case <-sigchan: // Wait for interrupt triggered via `k8sdebug logs runner stop`
			cancel()
			wg.Done()
		}
	}()
	wg.Wait()
	fmt.Println("Stopping logger...")
}

// Process pod first synchronously append metadata to the metadata log because order is important.
// And then starts a go routine that watches for pod logs and writes to log files.
func processPod(ctx context.Context, cs *kubernetes.Clientset, pod *v1.Pod, namespace string) {
	creationTime := pod.CreationTimestamp.Time
	podName := pod.Name
	// podNs := pod.Namespace
	// TODO: Fix this 5 second wait
	time.Sleep(time.Second * 5) // Wait for the pod to be ready
	fmt.Println("New pod added: " + pod.Name + "on time " + creationTime.Format("2006-01-02 15:04:05"))
	dir := filepath.Join(pkg.ConfigData.LogsPath, namespace)
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Println(err.Error())
		return
	}
	owner := getLastNode(&PodNode{
		pod: pod,
		cs:  cs,
	})
	path := filepath.Join(pkg.ConfigData.LogsPath, namespace, fmt.Sprintf("%s.%s.metadata", strings.ToLower(owner.Type()), owner.Name()))

	// Use the advisory lock to write in metadata file
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Println(err.Error())
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		fmt.Println("Error locking file:", err)
		return
	}
	f.WriteString(fmt.Sprintf("%s ; %s\n", creationTime.Format("2006-01-02 15:04:05"), podName))
	syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	f.Close()

	//TODO: Can there be a race condition here?
	checkpointData.LastResourceVersion = pod.ResourceVersion
	//Start watching and recording logs
	go func(podName string) {
		fmt.Println("Watching logs for pod: " + podName)
		path = filepath.Join(dir, podName+".log")
		file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			fmt.Println(err.Error())
		}
		defer file.Close() // Remember to close the file
		for {
			opts := &v1.PodLogOptions{
				Follow: true,
			}
			req := cs.CoreV1().Pods(namespace).GetLogs(podName, opts)
			stream, err := req.Stream(ctx)
			if err != nil {
				fmt.Println(err.Error())
				return
			}
			defer stream.Close()
			if _, err := io.Copy(file, stream); err != nil {
				fmt.Println(err.Error())
			}
			fmt.Println("Stream closed for pod: " + podName)
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

func getLastNode(n Node) Node {
	for {
		nextNode := n.Next()
		if nextNode == nil {
			return n
		}
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
			fmt.Println(fmt.Sprintf("Error fetching ReplicaSet %s: %v", name, err))
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
			fmt.Println(fmt.Sprintf("Error fetching ReplicaSet %s: %v", name, err))
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
