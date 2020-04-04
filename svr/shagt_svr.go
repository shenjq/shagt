package main

import (
	"flag"
	"fmt"
	"github.com/golang/glog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"shagt/comm"
	"shagt/conf"
	"shagt/etcd"
	"shagt/pub"
	"shagt/svr/svrbusi"
)

//var gCurrentPath string
var gWorkPath string
var gConfFile string    //配置文件路径
var Version = ""
var BuildTime = ""

func init() {
	isVer := flag.Bool("ver", false, "print program build version")
	isDaemon := flag.Bool("d", false, "run app as a daemon with -d=true")
	var err error
	gWorkPath, err = comm.GetWorkPath()
	if err != nil {
		fmt.Printf("GetWorkPath err,%v\n", err)
		os.Exit(0)
	}
	defaultCfgFile := fmt.Sprintf("%sconf/server-config.ini", gWorkPath)
	flag.StringVar(&gConfFile, "cfgfile", defaultCfgFile, "config file")

	if !flag.Parsed() {
		flag.Parse()
	}

	if *isVer {
		fmt.Printf("Version  : %s \n", Version)
		fmt.Printf("BuildTime: %s \n", BuildTime)
		os.Exit(0)
	}

	//参数检查
	err = checkPara()
	if err != nil {
		fmt.Printf("参数检查错误,%v", err)
		os.Exit(0)
	}

	//后台运行
	if *isDaemon {
		args := os.Args[1:]
		i := 0
		for ; i < len(args); i++ {
			if args[i] == "-d=true" {
				args[i] = "-d=false"
				break
			}
		}
		cmd := exec.Command(os.Args[0], args...)
		cmd.Start()
		fmt.Println("[PID]", cmd.Process.Pid)
		os.Exit(0)
	}
}

func checkPara() error {

	ok, err := pub.IsFile(gConfFile)
	if err != nil || !ok {
		fmt.Printf("配置文件%s不存在!\n", gConfFile)
		return fmt.Errorf("配置文件%s不存在!\n", gConfFile)
	}
	args := os.Args[1:]
	i := 0
	var loglevel, logdir bool
	for ; i < len(args); i++ {
		if args[i] == "-v" {
			loglevel = true
			continue
		}
		if args[i] == "-log_dir" {
			logdir = true
			continue
		}
	}
	if !loglevel {
		fmt.Printf("** 未设置日志级别，默认值level=0\n")
	}
	if !logdir {
		fmt.Printf("** 未设置日志目录!!\n")
	}
	return nil
}

func main() {
	flag.Parse()

	//读取配置文件
	err := conf.InitSerConf(gConfFile)
	if err != nil {
		glog.V(0).Infof("InitSerConf err,%v", err)
		return
	}
	glog.V(3).Infof("InitSerConf:%v", conf.GetSerConf())

	//etcd 连接，设置，监听key变化读取客户端注册清单
	etcdcli, err := etcd.NewEtcdClient([]string{conf.GetSerConf().EtcdAddress})
	if err != nil {
		glog.V(0).Infof("etcd.NewEtcdClient err,%v", err)
		return
	}
	if err = svrbusi.PutSerConf(etcdcli); err != nil {
		glog.V(0).Infof("PutSerConf err,%v", err)
		return
	}
	if err = svrbusi.DiscoverSer(etcdcli); err != nil {
		glog.V(0).Infof("DiscoverSer err,%v", err)
		return
	}

	go svrbusi.FinishHandle()

	//start web-server
	glog.V(0).Info("start server ...")
	mux := http.NewServeMux()
	mux.HandleFunc("/help", pub.Middle(help))
	mux.HandleFunc("/op", pub.Middle(svrbusi.Svr_handler))
	mux.HandleFunc("/query", pub.Middle(svrbusi.Query))
	mux.HandleFunc("/getinfo", pub.Middle(svrbusi.GetInfo))
	mux.HandleFunc("/download", pub.Middle(svrbusi.DownloadFile))
	mux.HandleFunc("/updatecm", pub.Middle(svrbusi.Upcm_handler))
	err = http.ListenAndServe("0.0.0.0:7788", mux)
	if err != nil {
		glog.V(0).Infof("start server err,%v", err)
		return
	}
}

func help(w http.ResponseWriter, r *http.Request) {
	hostaddr := "http://ip:7788"
	fmt.Fprintf(w, "主机监控管理,接收GET/POST请求,支持form格式.\n")
	fmt.Fprintf(w, "Usage:\n")
	fmt.Fprintf(w, "查询当前注册客户端列表: %s/query 接受host，支持模糊查询\n", hostaddr) //2.22
	fmt.Fprintf(w, "操作客户端: %s/op  接受host,action 等\n", hostaddr)
	fmt.Fprintf(w, "示例:\n")
	fmt.Fprintf(w, "查询客户端注册信息:")
	fmt.Fprintf(w, "\tcurl %s/query?host=CA3001 \n", hostaddr)
	fmt.Fprintf(w, "\tcurl %s/query?host=3001 \n", hostaddr)
	fmt.Fprintf(w, "\tcurl %s/query?host=192.168.0 \n", hostaddr)
	fmt.Fprintf(w, "获取指定客户端的服务器信息:")
	fmt.Fprintf(w, "\tcurl %s/getinfo?host=CA3001' \n", hostaddr)

	//关闭，重启
	fmt.Fprintf(w, "客户端程序重启:")
	fmt.Fprintf(w, "\tcurl -X POST %s/op -d 'host=CA3001&action=stop/start/reboot' \n", hostaddr)
	//升级
	fmt.Fprintf(w, "客户端程序升级,未指定版本则最新版本:")
	fmt.Fprintf(w, "\tcurl -X POST %s/op -d 'host=CA3001&action=update&ver=0.2' \n", hostaddr)
	//客户端fw
	//收集配置信息
	fmt.Fprintf(w, "更新指定客户端的服务器信息到cmdb:")
	fmt.Fprintf(w, "\tcurl -X POST %s/op -d 'host=CA3001&action=updatecm' \n", hostaddr)
	//检查文件更新
	fmt.Fprintf(w, "检查客户端的服务器容灾文件变动情况:")
	fmt.Fprintf(w, "\tcurl -X POST %s/op -d 'host=CA3001&action=checkfile' \n", hostaddr)

}

func RemoteIp(req *http.Request) string {
	remoteAddr := req.RemoteAddr

	if ip := req.Header.Get("Remote_addr"); ip != "" {
		remoteAddr = ip
	} else {
		remoteAddr, _, _ = net.SplitHostPort(remoteAddr)
	}

	if remoteAddr == "::1" {
		remoteAddr = "127.0.0.1"
	}
	glog.V(3).Infof("clinet ip:[%s]", remoteAddr)
	return remoteAddr
}
