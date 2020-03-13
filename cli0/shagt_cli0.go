package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/golang/glog"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"shagt/comm"
	"shagt/pub"
	"strings"
	"syscall"
	"time"
)

type handle struct {
	host string
	port string
}

const CLI1NAME string = "shagt_cli1"

var gCurrentPath string
var gWorkPath string
var gConfFile string
var Version = ""
var BuildTime = ""

var gCh_Cli1Stop = make(chan struct{}, 1)
var gCh_Cli1Start = make(chan struct{}, 1)
var gCh_Cli1ReStart = make(chan struct{}, 1)
var gCh_Cli1StopDone = make(chan error, 1)
var gCh_Cli1StartDone = make(chan error, 1)
var gCh_Cli1ReStartDone = make(chan error, 1)
var gServerAddress string
var gCli1Pid int

func init() {
	isVer := flag.Bool("ver", false, "print program build version")
	isDaemon := flag.Bool("d", false, "run app as a daemon with -d=true")
	var err error
	gWorkPath, err = comm.GetWorkPath()
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

func checkPara() (err error) {
	gCurrentPath, err = comm.GetCurrentPath()
	if err != nil {
		fmt.Printf("GetWorkPath err,%v\n", err)
		os.Exit(0)
	}
	ok, err := pub.IsFile(gCurrentPath + CLI1NAME)
	if err != nil || !ok {
		fmt.Printf("文件%s不存在!\n", gCurrentPath+CLI1NAME)
		return fmt.Errorf("文件%s不存在!\n", gCurrentPath+CLI1NAME)
	}
	ok, err = pub.IsFile(gConfFile)
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

func init() {
	gCh_Cli1Start <- struct{}{}
}

func main() {
	flag.Parse()

	//go startCli0()
	go OpCli1()

	go finishHandle()

	//被代理的服务器host和port
	target := &handle{host: "127.0.0.1", port: "7790"}
	err := http.ListenAndServe("0.0.0.0:7789", target)
	if err != nil {
		glog.V(0).Infof("ListenAndServe err, %v", err)
	}
}

func (this *handle) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var result pub.Resp
	var act string

	urlStr := r.URL.String()
	glog.V(0).Infof("Url :%s", urlStr)

	//'/op'接收post请求
	if urlStr == "/op" || urlStr == "/monitor" {
		remote, err := url.Parse("http://" + this.host + ":" + this.port)
		if err != nil {
			glog.V(0).Infof("url.Parse err,%v", err)
			panic(err)
		}
		proxy := httputil.NewSingleHostReverseProxy(remote)
		proxy.ServeHTTP(w, r)
		return
	}

	//内部定义'/op2'用于操作cli1
	err := r.ParseForm()
	if err != nil {
		result.Code = "401"
		result.Msg = "参数有误,未定义参数."
		result.Resp(w)
		return
	}
	stmp, ok := r.Form["action"]
	if !ok {
		result.Code = "401"
		result.Msg = "参数有误,请输入action参数."
	}

	act = strings.ToLower(stmp[0])
	glog.V(3).Infof("action :%s", act)
	if act == "stop" {
		gCh_Cli1Stop <- struct{}{}
		err := <-gCh_Cli1StopDone
		if err != nil {
			result.Code = "401"
			result.Msg = err.Error()
		} else {
			result.Code = "200"
			result.Msg = "success."
		}
	}
	if act == "start" {
		gCh_Cli1Start <- struct{}{}
		err := <-gCh_Cli1StartDone
		if err != nil {
			result.Code = "401"
			result.Msg = err.Error()
		} else {
			result.Code = "200"
			result.Msg = "success."
		}
	}

	if act == "restart" {
		gCh_Cli1ReStart <- struct{}{}
		err := <-gCh_Cli1ReStartDone
		if err != nil {
			result.Code = "401"
			result.Msg = err.Error()
		} else {
			result.Code = "200"
			result.Msg = "success."
		}
	}

	if act == "update" {
		ver := ""
		verTmp, ok := r.Form["ver"]
		if ok {
			ver = verTmp[0]
		}
		//stopcli1
		gCh_Cli1Stop <- struct{}{}
		<-gCh_Cli1StopDone
		//download cli1
		err := updateCli1(ver)
		if err != nil {
			result.Code = "401"
			result.Msg = err.Error()
			result.Resp(w)
			return
		}
		//startcli1
		gCh_Cli1Start <- struct{}{}
		err = <-gCh_Cli1StartDone
		if err != nil {
			result.Code = "401"
			result.Msg = err.Error()
		} else {
			result.Code = "200"
			result.Msg = "success."
		}
	}

	result.Resp(w)
}

func finishHandle() {
	err := <-gCh_Cli1StartDone
	glog.V(0).Infof("start cli1 done,%v", err)
}

func startCli0() {
	//被代理的服务器host和port
	target := &handle{host: "127.0.0.1", port: "7790"}
	err := http.ListenAndServe("0.0.0.0:7789", target)
	if err != nil {
		glog.V(0).Infof("ListenAndServe err, %v", err)
	}
}

func OpCli1() {
	for {
		select {
		case <-gCh_Cli1Stop:
			err := stopCli1()
			gCh_Cli1StopDone <- err
		case <-gCh_Cli1Start:
			err := startCli1()
			gCh_Cli1StartDone <- err
		case <-gCh_Cli1ReStart:
			stopCli1()
			time.Sleep(time.Second * 2)
			err := startCli1()
			gCh_Cli1ReStartDone <- err
		}
	}
}
func stopCli1() error {
	err := checkCli1(0)
	if err != nil {
		return nil
	}

	return syscall.Kill(gCli1Pid, syscall.SIGKILL)
}

func startCli1() error {
	err := checkCli1(0)
	if err == nil {
		return nil
	}

	//cmdStr := fmt.Sprintf("%s%s -d=true -v 4 -log_dir %slog", gCurrentPath, CLI1NAME, gCurrentPath)
	//pub.ExecOSCmd(cmdStr)
	args := os.Args[1:]
	i := 0
	for ; i < len(args); i++ {
		if args[i] == "-d=false" {
			args[i] = "-d=true"
			break
		}
	}
	cmd := exec.Command(gCurrentPath+CLI1NAME, args...)
	cmd.Start()
	cmd.Wait()
	glog.V(0).Infof("%s ....done", gCurrentPath+CLI1NAME)

	err = checkCli1(2)
	if err != nil {
		glog.V(3).Infof("checkCli1 err,%v", err)
		return fmt.Errorf("startCli1 failed!")
	}

	return nil
}

func checkCli1(RetryNum int) (err error) {
	type Cli1Resp struct {
		ServerAddress string
		Pid           int
	}
	var resp *Cli1Resp
	var str_ret string
	for i := 0; i <= RetryNum; i++ {
		str_ret, err = pub.Get("http://127.0.0.1:7790/check")
		if err != nil {
			time.Sleep(time.Second)
			continue
		} else {
			break
		}
	}
	if err != nil {
		return
	}
	glog.V(0).Infof("====>result:%s", str_ret)
	err = json.Unmarshal([]byte(strings.TrimSpace(str_ret)), &resp)
	if err == nil {
		gServerAddress = resp.ServerAddress
		gCli1Pid = resp.Pid
	}
	return
}

func updateCli1(ver string) error {
	urlStr := fmt.Sprintf("http://%s:7788/download?filename=%s%s",
		gServerAddress, CLI1NAME, strings.TrimSpace(ver))
	localPath := gCurrentPath + CLI1NAME

	return downloadFile(urlStr, localPath)
}

func downloadFile(url string, localPath string) (err error) {
	var (
		buf     = make([]byte, 10*1024)
		written int64
	)
	glog.V(3).Infof("url:%s", url)
	glog.V(3).Infof("localPath:%s", localPath)

	client := new(http.Client)
	//client.Timeout = time.Second * 60 //设置超时时间
	resp, err := client.Get(url)
	if err != nil {
		return err
	}

	if resp.Header.Get("Content-Type") != "application/octet-stream" {
		if resp.Body != nil {
			resp.Body.Read(buf)
		}
		return fmt.Errorf("下载文件失败,%s", string(buf))
	}
	if resp.Body == nil {
		return fmt.Errorf("body is null")
	}
	defer resp.Body.Close()

	tmpFilePath := localPath + ".download"
	file, err := os.Create(tmpFilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	for {
		nr, er := resp.Body.Read(buf)
		if nr > 0 {
			nw, ew := file.Write(buf[0:nr]) //写入bytes
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil { //写入出错
				err = ew
				break
			}
			//读取是数据长度不等于写入的数据长度
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
	if err != nil {
		return
	}
	return os.Rename(tmpFilePath, localPath)
}
