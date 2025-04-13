package portforward

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"

	"github.com/revolyssup/k8sdebug/pkg"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

var (
	namespace     string
	typ           string
	hostport      string
	containerPort string
)
var labels string
var startingHostPort = 8080

func forwardToPod(hostConn net.Conn, podCon net.Conn) {
	//Copy data bidirectionally
	go io.Copy(hostConn, podCon)
	io.Copy(podCon, hostConn)
}

var connPool []string

var policy string

func getPodConnection(fw forwarder) (net.Conn, error) {
	podConn, err := net.Dial("tcp", fmt.Sprintf(":%d", fw.Port()))
	if err != nil {
		return nil, err
	}
	return podConn, nil
}

type forwarder interface {
	Port() int
}

type roundRobin struct {
	connNumber int
}

func (rr *roundRobin) Port() int {
	portNum := 8080 + (rr.connNumber % len(connPool))
	rr.connNumber++
	return portNum
}
func getForwarder(policy string) forwarder {
	switch policy {
	case "round-robin":
		return &roundRobin{}
	}
	return nil
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
			defer listener.Close()
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
			initialList, err := cs.CoreV1().Pods(namespace).List(context.TODO(), opts)
			if err != nil {
				panic("Exiting runner..." + err.Error())
			}
			connPool = make([]string, 0)
			pods := initialList.Items
			fmt.Printf(pkg.ColorLine(fmt.Sprintf("listening on %s using %s policy across %d pods\n", hostport, policy, len(pods)), pkg.ColorGreen))
			for i, pod := range pods {
				req := cs.CoreV1().RESTClient().Post().
					Resource("pods").
					Namespace(namespace).
					Name(pod.Name).
					SubResource("portforward")
				transporter, upgrader, err := spdy.RoundTripperFor(config)
				if err != nil {
					fmt.Println("coudnot open connection for pod", pod.Name)
					continue
				}
				dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transporter}, "POST", req.URL())
				stopChan := make(chan struct{})
				readyChan := make(chan struct{})
				hostPort := startingHostPort + i
				hostPortStr := fmt.Sprintf("%s:%s", strconv.Itoa(hostPort), containerPort)
				connPool = append(connPool, hostPortStr)
				forwarder, err := portforward.New(dialer, []string{hostPortStr}, stopChan, readyChan, os.Stdout, os.Stderr)
				if err != nil {
					fmt.Println("coudnot forward connection for pod", pod.Name, " :", err.Error())
					continue
				}
				go func() {
					if err := forwarder.ForwardPorts(); err != nil {
						// errChan <- fmt.Errorf("port forwarding failed: %v", err)
						fmt.Println("coudnot forward connection for pod", pod.Name)
					}
				}()
			}
			var wg sync.WaitGroup
			sigchan := make(chan os.Signal)
			signal.Notify(sigchan, os.Interrupt)
			wg.Add(1)
			go func() {
				<-sigchan
				wg.Done()
				listener.Close()
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
