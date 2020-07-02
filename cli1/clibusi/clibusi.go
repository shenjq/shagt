package clibusi

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/robfig/cron"
	"math/rand"
	"net/http"
	"os"
	"runtime"
	"shagt/cli1/clilogmon"
	"shagt/cli1/ps"
	"shagt/comm"
	"shagt/conf"
	"shagt/etcd"
	"shagt/pub"
	"strconv"
	"strings"
	"time"
)

var gSerConf *comm.ReadFromServerConf

func GetServerConf() *comm.ReadFromServerConf {
	return gSerConf
}

func GetParaFormEtcd(client *etcd.EtcdClient) {
	for {
		select {
		case <-comm.G_ExecSque.Ch_GetParaFormEtcdStart:
			DoGetParaFormEtcd(client)
			comm.G_ExecSque.Ch_GetParaFormEtcdDone <- struct{}{}
		}
	}
}

func HttpServerCheck() {
	timeout := time.After(3 * time.Second)
	select {
	case <-timeout:
		glog.V(0).Info("GetParaFormEtcd failed")
		fmt.Fprintln(os.Stderr, "*panic: GetParaFormEtcd failed!")
		glog.Flush()
		os.Exit(0)
	case <-comm.G_ExecSque.Ch_GetParaFormEtcdDone:
		glog.V(0).Info("GetParaFormEtcdDone success")
	}
	for {
		glog.V(0).Info("checking cli1 has started yet ?")
		time.Sleep(time.Second)
		resp, _ := pub.Get("http://127.0.0.1:7790/help")
		if len(strings.TrimSpace(resp)) == 0 {
			continue
		} else {
			glog.V(0).Info("cli1 has start success")
			comm.G_ExecSque.Ch_CliRegStart <- struct{}{}
			comm.G_ExecSque.Ch_CliLogMonStart <- struct{}{}
			comm.G_ExecSque.Ch_ConnectCMStart <- struct{}{}
			comm.G_ExecSque.Ch_CheckFileStart <- struct{}{}
			break
		}
	}
}

func CliReg(client *etcd.EtcdClient) {
	select {
	case <-comm.G_ExecSque.Ch_CliRegStart:
		DoCliReg(client)
	}
}

//监控日志文件，在程序启动时注册后台运行
func CliLogMon() {
	select {
	case <-comm.G_ExecSque.Ch_CliLogMonStart:
		if err := clilogmon.CliLogMonInit(); err != nil {
			glog.V(0).Infof("CliLogMonInit err,%v", err)
			return
		}
	}
}

func ConnectCM() {
	select {
	case <-comm.G_ExecSque.Ch_ConnectCMStart:
		ss := ps.GetMachineInfo()
		ps.CheckCM(ss)
	}
}

//监控容灾文件变化情况，每日定期执行，更改文件内容无需重启
func CheckDrFile() {
	for {
		select {
		case <-comm.G_ExecSque.Ch_CheckFileStart:
			result, err := pub.CheckFile(conf.GetCliConf().DrCheckFilePath)
			comm.G_ExecSque.Ch_CheckFileDone <- struct{}{}
			if err != nil {
				glog.V(0).Infof("CheckDrFile err:%v", err)
			} else {
				syncDrFile(result)
			}
		}
	}
}

//处理发生变动的文件的同步操作
func syncDrFile(stru *[]pub.FileMd5Stru) {
	glog.V(0).Infof("发生变动的文件列表:")
	for _, v := range *stru {
		glog.V(0).Infof("%s,%s,%s", v.Filepath, v.Md5str, v.Note)
	}
}

func FinishHandle() {
	<-comm.G_ExecSque.Ch_CheckFileDone
	addCheckFileScheme()
}

//客户端读取server端设置的参数
func DoGetParaFormEtcd(client *etcd.EtcdClient) error {
	//从配置文件读取key
	cliconf := conf.GetCliConf()
	//从etcd读取server/cfg/
	serconf, err := client.GetKey("server/cfg/")
	if err != nil {
		glog.V(0).Infof("GetKey err:%v", err)
		return err
	}
	gSerConf = &comm.ReadFromServerConf{}
	if v, ok := (*serconf)[cliconf.ServerAddress_key]; ok {
		gSerConf.ServerAddress = v
	} else {
		glog.V(0).Infof("get serverAddress from etcd err.")
		return fmt.Errorf("get serverAddress from etcd err")
	}
	if v, ok := (*serconf)[cliconf.CmdbAddress_key]; ok {
		gSerConf.CmdbAddress = v
	} else {
		glog.V(0).Infof("get CmdbAddress from etcd err.")
		gSerConf.CmdbAddress = ""
		//return fmt.Errorf("get CmdbAddress from etcd err")
	}
	if v, ok := (*serconf)[cliconf.ECAddress_key]; ok {
		gSerConf.ECAddress = v
	} else {
		glog.V(0).Infof("get ECAddress from etcd err.")
		gSerConf.ECAddress = ""
		//return fmt.Errorf("get ECAddress from etcd err")
	}
	if v, ok := (*serconf)[cliconf.CliTtl_key]; ok {
		gSerConf.CliTtl, _ = strconv.Atoi(v)
	}
	if gSerConf.CliTtl == 0 {
		glog.V(0).Infof("get CliTtl from etcd err.")
		gSerConf.CliTtl = 30
	}
	if v, ok := (*serconf)[cliconf.SoftCheck_key]; ok {
		gSerConf.SoftCheck = v
	} else {
		glog.V(0).Infof("get SoftChecklist from etcd err.")
	}
	comm.G_ReadFromServerConf = gSerConf
	return nil
}

func DoCliReg(client *etcd.EtcdClient) error {
	serreg, err := client.NewServiceReg(gSerConf.CliTtl)
	if err != nil {
		glog.V(0).Infof("NewServiceReg err,%v", err)
		return err
	}
	cliconf := conf.GetCliConf()
	key := "cli/reg/" + cliconf.LocalHostName
	//hostname，ip，pid，version，os
	value := fmt.Sprintf("%s,%s,%d,%s,%s",
		cliconf.LocalHostName, cliconf.LocalHostIp, os.Getpid(), comm.G_CliInfo.Version, runtime.GOOS)

	err = serreg.PutService(key, value)
	if err != nil {
		glog.V(0).Infof("PutService err,%v", err)
		return err
	}
	return nil
}

func addCheckFileScheme() {
	glog.V(1).Info("设定文件检查后台定时任务...")
	c := cron.New()
	if c == nil {
		glog.V(0).Info("cron.new err")
		return
	}
	//定时任务时间，暂定每天7:00--8:00
	rand.Seed(time.Now().UnixNano())
	spec := fmt.Sprintf("%d 7 * * *", rand.Intn(59))
	glog.V(1).Infof("容灾文件变化检查执行时间:%s", spec)
	var err error
	err = c.AddFunc(spec, func() {
		pub.CheckFile(conf.GetCliConf().DrCheckFilePath)
	})
	if err != nil {
		glog.V(0).Infof("addfunc err,%v", err)
		return
	}
	spec2 := fmt.Sprintf("%d 6 * * *", rand.Intn(59))
	glog.V(1).Infof("更新配置文件执行时间:%s", spec2)
	err = c.AddFunc(spec, func() {
		ss := ps.GetMachineInfo()
		ps.CheckCM(ss)
	})
	if err != nil {
		glog.V(0).Infof("addfunc err,%v", err)
		return
	}

	c.Start()
	defer c.Stop()
	select {}
}

func Op(w http.ResponseWriter, r *http.Request) {
	var result pub.Resp
	var act string

	urlStr := r.URL.String()
	glog.V(0).Infof("cli1 receive request Url :%s", urlStr)

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
	} else {
		act = strings.ToLower(stmp[0])
		glog.V(3).Infof("action :%s", act)
		if act == "updatecm" {
			if err := updateCM(); err != nil {
				result.Code = "401"
				result.Msg = err.Error()
			} else {
				result.Code = "200"
				result.Msg = "success."
			}
		} else if act == "checkfile" {
			timeout := time.After(3 * time.Second)
			glog.V(3).Infof("send cmd -> CheckFile ...")
			comm.G_ExecSque.Ch_CheckFileStart <- struct{}{}
			select {
			case <-comm.G_ExecSque.Ch_CheckFileDone:
				glog.V(3).Infof("send cmd -> CheckFile ... Done")
				result.Code = "200"
				result.Msg = "success."
			case <-timeout:
				result.Code = "401"
				result.Msg = "timeout."
			}
		} else {
			result.Code = "401"
			result.Msg = "参数有误,未定义参数."
		}
	}
	result.Resp(w)
}

func updateCM() error {
	timeout := time.After(3 * time.Second)
	glog.V(3).Infof("send cmd -> updateCM,GetParaFormEtcdStart ...")
	comm.G_ExecSque.Ch_GetParaFormEtcdStart <- struct{}{}
	select {
	case <-comm.G_ExecSque.Ch_GetParaFormEtcdDone:
		glog.V(3).Infof("getparaformetcd success,start GetMachineInfo ...")
		ss := ps.GetMachineInfo()
		ps.CheckCM(ss)
		return nil
	case <-timeout:
		glog.V(3).Infof("*** getparaformetcd timeout !!!")
		return fmt.Errorf("getparaformetcd timeout!")
	}
}

func Monitor(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Add("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	sm := ps.GetMachineInfo()
	b, err := json.MarshalIndent(sm, "", "    ")
	if err != nil {
		fmt.Fprintf(w, "获取监控信息失败")
	} else {
		fmt.Fprintf(w, string(b))
	}
}

func Check(w http.ResponseWriter, r *http.Request) {
	type Cli1Resp struct {
		ServerAddress string
		Pid           int
	}
	var resp = Cli1Resp{
		ServerAddress: comm.G_ReadFromServerConf.ServerAddress,
		Pid:           os.Getpid(),
	}

	b, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(w, "json.Marshal err,%v", err)
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	}
}
