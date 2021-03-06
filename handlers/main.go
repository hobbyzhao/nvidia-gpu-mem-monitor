package handlers

import (
	"fmt"
	"net"
	"os"

	"github.com/docker/docker/client"
	svc "github.com/zlingqu/nvidia-gpu-mem-monitor/service"
)

func Metrics() string {

	cli, err := client.NewClient("unix:///var/run/docker.sock", "v1.37", nil, nil) //使用socket通信
	defer cli.Close()                                                              //记得释放
	if err != nil {
		panic(err)
	}

	records := svc.GetExecOutByCSV("nvidia-smi --query-compute-apps=pid,used_gpu_memory,gpu_name,gpu_uuid --format=csv,noheader,nounits")

	var response string = `# HELP pod_used_gpu_mem_MB . Pod使用的GPU显存大小
# TYPE pod_used_gpu_mem_MB gauge
`

	for _, row := range records {
		cmd := "cat /proc/" + row[0] + "/cgroup |head -1 | awk -F'/' '{print $5}'"
		containID := svc.GetExecOutByString(cmd)
		podName, podNamespace := "null", "null" //非pod使用gpu的进程
		if containID != "" {
			podName, podNamespace = svc.GetContainsPodInfo(cli, containID) //获取pod信息
		}
		response = fmt.Sprintf("%spod_used_gpu_mem_MB{instance=\"%s\",app_pid=\"%s\",gpu_name=\"%s\",gpu_uuid=\"%s\",pod_name=\"%s\",pod_namespace=\"%s\"} %s\n",
			response, getIP(), row[0], row[2], row[3], podName, podNamespace, row[1])
	}
	return response
}

func getIP() string {
	if hostIP := os.Getenv("hostIP"); hostIP != "" { //如果部署到k8s中会注入hostIP变量
		return hostIP
	}
	netInterfaces, err := net.Interfaces()
	if err != nil {
		fmt.Println("net.Interfaces failed, err:", err.Error())
		return ""
	}
	for i := 0; i < len(netInterfaces); i++ {
		//fmt.Println(netInterfaces[i],net.FlagUp)
		if (netInterfaces[i].Flags&net.FlagUp) != 0 && interFaceFields(netInterfaces[i]) {
			adds, _ := netInterfaces[i].Addrs()

			for _, address := range adds {
				//fmt.Println(address)
				if inet, ok := address.(*net.IPNet); ok && !inet.IP.IsLoopback() {
					if inet.Contains(inet.IP) && inet.IP.To4() != nil {
						return inet.IP.String()
					}
				}
			}
		}
	}
	return ""
}

func interFaceFields(myInterFace net.Interface) bool {
	if myInterFace.MTU != 1500 {
		return false
	}
	if len(myInterFace.HardwareAddr) > 17 { //排查ib网络的网卡
		return false
	}
	for _, v := range []string{"cni0", "flannel.1", "docker0", "virbr0"} { //排除特殊的网卡设备
		if myInterFace.Name == v {
			return false
		}
	}
	return true
}
