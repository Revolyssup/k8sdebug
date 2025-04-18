package portforward

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"

	"github.com/revolyssup/k8sdebug/pkg"
	"github.com/revolyssup/k8sdebug/pkg/forwarder"
	"github.com/revolyssup/k8sdebug/pkg/portforward/mock"
	"github.com/revolyssup/k8sdebug/pkg/portforward/roundrobin"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

var startingHostPort = 8080
var (
	namespace          string
	typ                string
	hostport           string
	containerPort      string
	labels             string
	policy             string
	connPool           []string
	freeList           = make([]int, 0)
	podNameToPoolIndex map[string]int
	fromWatch          = -1
	indexToStopChan    = make(map[int]chan struct{}) // Track stop channels by index to close previous port forwards
)

func forwardToPod(hostConn net.Conn, podCon net.Conn) {
	//Copy data bidirectionally
	go io.Copy(hostConn, podCon)
	io.Copy(podCon, hostConn)
}

func getPodConnection(fw forwarder.Forwarder) (net.Conn, error) {
	port := fw.NextPort()
	podConn, err := net.Dial("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		return nil, err
	}
	return podConn, nil
}

func getForwarder(policy string) forwarder.Forwarder {
	switch policy {
	case "round-robin":
		return roundrobin.New(connPool)
	case "mock":
		return mock.New()
	}
	return nil
}

var mx sync.Mutex

func getFreeIndex() int {
	mx.Lock()
	defer mx.Unlock()
	if len(freeList) != 0 {
		i := freeList[0]
		connPool[i] = ""
		freeList = freeList[1:]
		return i
	}
	connPool = append(connPool, "") //Placeholder to increase the size
	return len(connPool) - 1
}

func addFreeIndex(i int) {
	mx.Lock()
	defer mx.Unlock()
	freeList = append(freeList, i)
	connPool[i] = "" // RESET
}
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "port-forward",
		Run: func(cmd *cobra.Command, args []string) {
			fw := getForwarder(policy)
			if fw == nil {
				fmt.Errorf("invalid policy for forwarding traffic\n")
				return
			}
			listener, err := net.Listen("tcp", fmt.Sprintf(":%s", hostport))
			if err != nil {
				panic(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go func() {
				for {
					select {
					case <-ctx.Done():
						return
					default:
						hostConn, err := listener.Accept()
						if err != nil {
							if errors.Is(err, net.ErrClosed) {
								return // Prolly exiting
							}
							fmt.Printf("Accept error: %v", err)
							continue
						}
						podConn, err := getPodConnection(fw)
						if err != nil {
							fmt.Printf("Pod connection error: %v", err)
							continue
						}
						forwardToPod(hostConn, podConn)
					}

				}
			}()
			kubeconfig := clientcmd.NewDefaultClientConfigLoadingRules().GetDefaultFilename()
			config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
			if err != nil {
				panic(err.Error())
			}
			cs := kubernetes.NewForConfigOrDie(config)
			opts := metav1.ListOptions{}
			if labels != "" {
				opts.LabelSelector = labels
			}
			initialList, err := cs.CoreV1().Pods(namespace).List(ctx, opts)
			if err != nil {
				panic("Exiting runner..." + err.Error())
			}
			pods := initialList.Items
			connPool = make([]string, len(pods))
			podNameToPoolIndex = make(map[string]int, len(pods))
			fmt.Printf(pkg.ColorLine(fmt.Sprintf("listening on %s using %s policy across %d pods\n", hostport, policy, len(pods)), pkg.ColorGreen))
			startPortForward := func(i int, pod v1.Pod) (string, error) {
				if i == fromWatch {
					i = getFreeIndex()
				}
				if stopChan, exists := indexToStopChan[i]; exists {
					close(stopChan)
					delete(indexToStopChan, i)
				}
				fmt.Println("WILL TRY TO CREATE AT INDEX", i)
				req := cs.CoreV1().RESTClient().Post().
					Resource("pods").
					Namespace(namespace).
					Name(pod.Name).
					SubResource("portforward")
				transporter, upgrader, err := spdy.RoundTripperFor(config)
				if err != nil {
					fmt.Println("coudnot open connection for pod", pod.Name)
					return "", err
				}
				dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transporter}, "POST", req.URL())
				stopChan := make(chan struct{})
				indexToStopChan[i] = stopChan // Track new stop channel
				readyChan := make(chan struct{})
				hostPort := startingHostPort + i

				hostPortStr := fmt.Sprintf("%s:%s", strconv.Itoa(hostPort), containerPort)
				connPool[i] = strconv.Itoa(hostPort)
				podNameToPoolIndex[pod.Name] = i
				forwarder, err := portforward.New(dialer, []string{hostPortStr}, stopChan, readyChan, os.Stdout, os.Stderr)
				if err != nil {
					connPool[i] = "" // Reset connPool entry
					addFreeIndex(i)  // Return index to freeList
					return "", fmt.Errorf("port-forward setup failed: %v", err)
				}
				go func() {
					fmt.Println("FORWARDER WILL RUN FOR ", hostPortStr)
					if err := forwarder.ForwardPorts(); err != nil {
						fmt.Println("coud not forward connection for pod", pod.Name)
					}
				}()
				return hostPortStr, nil
			}
			for i, pod := range pods {
				startPortForward(i, pod)
			}
			opts.ResourceVersion = initialList.ResourceVersion
			watcher, err := cs.CoreV1().Pods(namespace).Watch(ctx, opts)
			if err != nil {
				fmt.Println("could not create a watcher for pods")
				return
			}
			var wg sync.WaitGroup
			go func() {
				for event := range watcher.ResultChan() {
					switch event.Type {
					case watch.Added:
						pod := event.Object.(*v1.Pod)
						if pod == nil {
							continue
						}
						fmt.Printf("new pod recieved: %s. will try to create portforward\n", pod.Name)
						//TODO: find a better way
						time.Sleep(2 * time.Second) //Wait for pod to start
						addr, err := startPortForward(fromWatch, *pod)
						if err != nil {
							fmt.Printf("could not create port forward for %s\n", pod.Name)
							continue
						}
						fmt.Println(pkg.ColorLine(fmt.Sprintf("New port forward created for pod %s on %s", pod.Name, addr), pkg.ColorGreen))
					case watch.Deleted:
						pod := event.Object.(*v1.Pod)
						i := podNameToPoolIndex[pod.Name]
						if stopChan, exists := indexToStopChan[i]; exists {
							close(stopChan) // Signal to stop port-forward
							delete(indexToStopChan, i)
						}
						addFreeIndex(i)
						fmt.Println(pkg.ColorLine("New freelist: ", pkg.ColorYellow), freeList)
					}
				}
			}()

			sigchan := make(chan os.Signal)
			signal.Notify(sigchan, os.Interrupt)
			wg.Add(1)
			go func() {
				<-sigchan
				wg.Done()
			}()
			wg.Wait()
			fmt.Println("Stopping watcher...")
		},
	}

	cmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "default", "Name of the pod")
	cmd.PersistentFlags().StringVarP(&typ, "type", "t", "pod", "Name of the pod")
	cmd.PersistentFlags().StringVar(&policy, "policy", "round-robin", "policy to use while sending requests")
	cmd.Flags().StringVarP(&labels, "labels", "l", "", "list of key value pairs to use as labels while filtering pods.")
	cmd.Flags().StringVar(&hostport, "hostport", "3000", "host port on which requests will be sent")
	cmd.Flags().StringVar(&containerPort, "containerport", "80", "container port on which requests will be sent")
	return cmd
}
