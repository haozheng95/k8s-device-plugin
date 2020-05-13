package main

import (
	"os"
	//"strconv"
	"bytes"
	"os/exec"
	//"syscall"
	"flag"
	"fmt"
	"github.com/golang/glog"
	"net"
	"path"
	"sync"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1alpha"
)

//const (
var onloadver string    //  = "201606-u1.3"
var onloadsrc string    //= "http://www.openonload.org/download/openonload-" + onloadver + ".tgz"
var regExpSFC string    //= "(?m)[\r\n]+^.*SFC[6-9].*$"
var socketName string   //= "sfcNIC"
var resourceName string //= "pod.alpha.kubernetes.io/opaque-int-resource-sfcNIC"

//)
// sfcNICManager manages Solarflare NIC devices
type sfcNICManager struct {
	devices     map[string]*pluginapi.Device
	deviceFiles []string
}

func NewSFCNICManager() (*sfcNICManager, error) {
	return &sfcNICManager{
		devices:     make(map[string]*pluginapi.Device),
		deviceFiles: []string{"/dev/onload", "/dev/onload_cplane", "dev/onload_epoll", "/dev/sfc_char", "/dev/sfc_affinity"},
	}, nil
}

func (sfc *sfcNICManager) Init() error {
	glog.Info("Init\n")
	return nil
}

func Register(kubeletEndpoint string, pluginEndpoint, socketName string) error {
	conn, err := grpc.Dial(kubeletEndpoint, grpc.WithInsecure(),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}))
	defer conn.Close()
	if err != nil {
		return fmt.Errorf("device-plugin: cannot connect to kubelet service: %v", err)
	}
	client := pluginapi.NewRegistrationClient(conn)
	reqt := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     pluginEndpoint,
		ResourceName: resourceName,
	}

	_, err = client.Register(context.Background(), reqt)
	if err != nil {
		return fmt.Errorf("device-plugin: cannot register to kubelet service: %v", err)
	}
	return nil
}

// Implements DevicePlugin service functions
func (sfc *sfcNICManager) ListAndWatch(emtpy *pluginapi.Empty, stream pluginapi.DevicePlugin_ListAndWatchServer) error {
	glog.Info("device-plugin: ListAndWatch start\n")
	for {
		glog.Info("device-plugin: ListAndWatch loop\n")
		time.Sleep(5 * time.Second)
	}
	return nil
}

func (sfc *sfcNICManager) Allocate(ctx context.Context, rqt *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	glog.Info("Allocate")
	resp := new(pluginapi.AllocateResponse)
	//	containerName := strings.Join([]string{"k8s", "POD", rqt.PodName, rqt.Namespace}, "_")
	for _, id := range rqt.DevicesIDs {
		if _, ok := sfc.devices[id]; ok {
			for _, d := range sfc.deviceFiles {
				resp.Devices = append(resp.Devices, &pluginapi.DeviceSpec{
					HostPath:      d,
					ContainerPath: d,
					Permissions:   "mrw",
				})
			}
			glog.Info("Allocated interface ", id)
			//glog.Info("Allocate interface ", id, " to ", containerName)
		}
	}
	return resp, nil
}

func (sfc *sfcNICManager) UnInit() {
	var out bytes.Buffer
	var stderr bytes.Buffer

	//fmt.Println("CMD--" + cmdName + ": " + out.String())
	cmdName := "onload_uninstall"
	cmd := exec.Command(cmdName)
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		fmt.Println("CMD--" + cmdName + ": " + fmt.Sprint(err) + ": " + stderr.String())
	}
	//fmt.Println("CMD--" + cmdName + ": " + out.String())

	return
}

func main() {
	flag.Parse()
	fmt.Printf("Starting main \n")

	onloadver = "201606-u1.3"
	//onloadsrc = os.Args[2]    //"http://www.openonload.org/download/openonload-" + onloadver + ".tgz"
	regExpSFC = "(?m)[\r\n]+^.*SFC[6-9].*$"
	socketName = "sfcNIC"
	resourceName = "pod.alpha.kubernetes.io/opaque-int-resource-sfcNIC"
	flag.Lookup("logtostderr").Value.Set("true")

	sfc, err := NewSFCNICManager()
	if err != nil {
		glog.Fatal(err)
		os.Exit(1)
	}

	pluginEndpoint := fmt.Sprintf("%s-%d.sock", socketName, time.Now().Unix())
	//serverStarted := make(chan bool)
	var wg sync.WaitGroup
	wg.Add(1)
	// Starts device plugin service.
	go func() {
		defer wg.Done()
		fmt.Printf("DveicePluginPath %s, pluginEndpoint %s\n", pluginapi.DevicePluginPath, pluginEndpoint)
		fmt.Printf("device-plugin start server at: %s\n", path.Join(pluginapi.DevicePluginPath, pluginEndpoint))
		lis, err := net.Listen("unix", path.Join(pluginapi.DevicePluginPath, pluginEndpoint))
		if err != nil {
			glog.Fatal(err)
			return
		}
		grpcServer := grpc.NewServer()
		pluginapi.RegisterDevicePluginServer(grpcServer, sfc)
		grpcServer.Serve(lis)
	}()

	// TODO: fix this
	time.Sleep(5 * time.Second)
	// Registers with Kubelet.
	err = Register(pluginapi.KubeletSocket, pluginEndpoint, resourceName)
	if err != nil {
		glog.Fatal(err)
	}
	fmt.Printf("device-plugin registered\n")
	wg.Wait()
}
