package main

import (
	"flag"
	"fmt"
	"github.com/golang/glog"
	"net/http"
	"os"
	"os/exec"
	"shagt/cli1/clibusi"
	"shagt/comm"
	"shagt/conf"
	"shagt/etcd"
	"shagt/pub"
)

//var gCurrentPath string //命令当前路径
var gWorkPath string
var gConfFile string //配置文件路径
var Version = ""
var BuildTime = ""

func init() {
	isVer := flag.Bool("ver", false, "print program build version")
	isDaemon := flag.Bool("d", false, "run app as a daemon with -d=true")
	var err error
	gWorkPath, err = pub.GetWorkPath()
	if err != nil {
		fmt.Printf("GetWorkPath err,%v\n", err)
		os.Exit(0)
	}
	defaultCfgFile := fmt.Sprintf("%sconf/client-config.ini", gWorkPath)
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

	comm.G_CliInfo.WorkPath = gWorkPath
	comm.G_CliInfo.ConfFile = gConfFile
	comm.G_CliInfo.Version = Version

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

func init() {
	comm.G_ExecSque = &comm.ExecSque{
		Ch_GetParaFormEtcdStart: make(chan struct{}, 1),
		Ch_GetParaFormEtcdDone:  make(chan struct{}),
		Ch_CliRegStart:          make(chan struct{}),
		Ch_CliLogMonStart:       make(chan struct{}),
		Ch_ConnectCMStart:       make(chan struct{}),
		Ch_CheckFileStart:       make(chan struct{}),
		Ch_CheckFileDone:        make(chan struct{}),
	}
	comm.G_ExecSque.Ch_GetParaFormEtcdStart <- struct{}{}
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
		fmt.Printf("** 未设置日志目录，不记录日志!!\n")
	}
	return nil
}

func main() {
	flag.Parse()
	defer glog.Flush()

	//读取配置文件
	err := conf.InitCliConf(gConfFile)
	if err != nil {
		glog.V(0).Infof("InitCliConf err,%v", err)
		return
	}
	glog.V(3).Infof("InitSerConf:%v", conf.GetCliConf())

	//etcd 连接，设置，监听key变化读取客户端注册清单
	etcdcli, err := etcd.NewEtcdClient([]string{conf.GetCliConf().EtcdAddress})
	if err != nil {
		glog.V(0).Infof("etcd.NewEtcdClient err,%v", err)
		return
	}
	glog.V(1).Info("GetParaFormEtcd ...")
	go clibusi.GetParaFormEtcd(etcdcli)

	glog.V(1).Info("reg server ...")
	go clibusi.CliReg(etcdcli)

	//监控日志文件
	glog.V(1).Info("mon logfile ...")
	go clibusi.CliLogMon()

	//检查重要文件变化,定期执行
	glog.V(1).Info("CheckDrFile ...")
	go clibusi.CheckDrFile()

	//收集配置信息
	glog.V(1).Info("ConnectCM ...")
	go clibusi.ConnectCM()

	//start web-server
	go clibusi.HttpServerCheck()

	go clibusi.FinishHandle()

	glog.V(1).Info("start cli1 server ...")
	mux := http.NewServeMux()
	mux.HandleFunc("/help", pub.Middle(help))
	mux.HandleFunc("/op", pub.Middle(clibusi.Op))
	mux.HandleFunc("/monitor", pub.Middle(clibusi.Monitor))
	mux.HandleFunc("/check", pub.Middle(clibusi.Check)) //获取服务器端地址、cli1进程号
	err = http.ListenAndServe("0.0.0.0:17790", mux)
	if err != nil {
		glog.V(0).Infof("start server err,%v", err)
		return
	}
}

func help(w http.ResponseWriter, r *http.Request) {
	//hostaddr := "http://ip:17790"
	fmt.Fprintf(w, "主机端agent程序cli1,接收POST请求,支持form格式.\n")
	fmt.Fprintf(w, "Usage:\n")
	fmt.Fprintf(w, "   /op 接受action=updatecm/checkfile\n")
	fmt.Fprintf(w, "   /monitor 主机信息。\n")
	fmt.Fprintf(w, "   /check   健康检查。\n")

	//升级
	//客户端fw
	//收集配置信息
}
