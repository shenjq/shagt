package svrbusi

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"io/ioutil"
	"net/http"
	"os"
	"shagt/conf"
	"shagt/etcd"
	"shagt/pub"
	"strings"
	"syscall"
	"time"
)

var gDisCli *etcd.ClientDis

var gCH_cm = make(chan string, 1024)

func DiscoverSer(client *etcd.EtcdClient) (err error) {
	gDisCli = client.NewClientDis()
	_, err = gDisCli.GetService("cli/")
	if err != nil {
		glog.V(0).Infof("GetService err,%v", err)
		return
	}
	go dispSer()
	return nil
}
func dispSer() {
	for ; ; {
		glog.V(3).Infof("---->regserver:%v", gDisCli.ServerList)
		time.Sleep(time.Second * 5)
	}
}
func Query(w http.ResponseWriter, r *http.Request) {
	var result pub.Resp
	var host string

	urlStr := r.URL.String()
	glog.V(0).Infof("host :%s", urlStr)

	r.ParseForm()
	stmp, ok := r.Form["host"]
	if !ok {
		host = ""
	} else {
		host = stmp[0]
	}
	cliList := gDisCli.SerList2Array(strings.TrimSpace(host))
	if len(cliList) == 0 {
		result.Code = "401"
		result.Msg = fmt.Sprintf("参数有误,请指定服务器")
		result.Resp(w)
	} else {
		for i, v := range cliList {
			fmt.Fprintf(w, "%03d %s\n", i+1, v)
		}
	}
}

func GetInfo(w http.ResponseWriter, r *http.Request) {
	var result pub.Resp
	var host string

	urlStr := r.URL.String()
	glog.V(0).Infof("host :%s", urlStr)

	err := r.ParseForm()
	if err != nil {
		result.Code = "401"
		result.Msg = "参数有误"
		result.Resp(w)
		return
	}
	stmp, ok := r.Form["host"]
	if !ok {
		result.Code = "401"
		result.Msg = "参数有误,请指定服务器。"
		result.Resp(w)
	} else {
		host = stmp[0]
		cliList := gDisCli.SerList2Array(strings.TrimSpace(host))
		if len(cliList) != 1 {
			result.Code = "401"
			result.Msg = fmt.Sprintf("参数有误,请指定服务器,查询服务器列表:%v", cliList)
			result.Resp(w)
		} else {
			cli := strings.Split(cliList[0], ",") //hostname,ip,pid,ver,os
			url := fmt.Sprintf("http://%s:7789/monitor", cli[1])
			mon, _ := pub.Get(url)
			fmt.Fprint(w, mon)
		}
	}
}

func DownloadFile(w http.ResponseWriter, r *http.Request) {
	var result pub.Resp
	var filename string

	urlStr := r.URL.String()
	glog.V(0).Infof("request :%s", urlStr)

	r.ParseForm()
	stmp, ok := r.Form["filename"]
	if !ok {
		result.Code = "401"
		result.Msg = "参数有误,未指定文件名称."
		result.Resp(w)
		return
	} else {
		filename = strings.TrimSpace(stmp[0])
		filepath := conf.GetSerConf().CliSoftPath + filename
		file, err := os.Open(filepath)
		if err != nil {
			result.Code = "401"
			result.Msg = fmt.Sprintf("open file err,%v", err)
			glog.V(0).Infof("open file err,%v", err)
			result.Resp(w)
			return
		}
		defer file.Close()
		content, err := ioutil.ReadAll(file)
		if err != nil {
			result.Code = "401"
			result.Msg = fmt.Sprintf("Read file err,%v", err)
			glog.V(0).Infof("Read file err,%v", err)
			result.Resp(w)
			return
		} else {
			w.Header().Add("Content-Type", "application/octet-stream")
			w.Write(content)
		}
	}
}

func getServerList(host string) (list []string) {
	list = make([]string, 0)
	for _, v := range gDisCli.ServerList {
		//glog.V(0).Infoln(v)
		if len(host) == 0 || strings.Contains(v.Hostname, host) {
			list = append(list, v.Hostname)
		}
	}
	return
}

func Svr_handler(w http.ResponseWriter, r *http.Request) {
	var result pub.Resp

	glog.V(0).Infof("request :%s", r.URL.String())

	tp := r.Header.Get("Content-Type")
	if strings.Contains(tp, "json") { //json格式
		result.Code = "401"
		result.Msg = "参数有误,暂不支持json格式"
		result.Resp(w)
		return
	}
	err := r.ParseForm()
	if err != nil {
		result.Code = "401"
		result.Msg = "参数有误"
		result.Resp(w)
		return
	}
	h, hok := r.Form["host"]
	a, aok := r.Form["action"]
	if !hok || !aok {
		result.Code = "401"
		result.Msg = "参数有误"
		result.Resp(w)
		return
	}

	host := h[0]
	action := strings.ToLower(a[0])
	var ver string
	if action == "update" {
		v, vok := r.Form["ver"]
		if !vok {
			ver = ""
		} else {
			ver = v[0]
		}
	}

	cliList := gDisCli.SerList2Array(strings.TrimSpace(host))
	if len(cliList) != 1 {
		result.Code = "401"
		result.Msg = fmt.Sprintf("参数有误,请指定服务器,查询服务器列表:%v", cliList)
		result.Resp(w)
		return
	} else {
		cli := strings.Split(cliList[0], ",") //hostname,ip,pid,ver
		var url string
		var data string
		if action == "start" || action == "stop" || action == "restart" {
			url = fmt.Sprintf("http://%s:7789/op2", cli[1])
			data = fmt.Sprintf("action=%s", action)
		} else if action == "update" {
			url = fmt.Sprintf("http://%s:7789/op2", cli[1])
			data = fmt.Sprintf("action=%s&ver=%s", action, ver)
		} else {
			url = fmt.Sprintf("http://%s:7789/op", cli[1])
			data = fmt.Sprintf("action=%s", action)
		}
		r, _ := pub.PostForm(url, data)
		fmt.Fprint(w, r)
	}
}

func Upcm_handler(w http.ResponseWriter, r *http.Request) {
	//var cm comm.CMStru
	var result pub.Resp
	var body_str string

	defer result.Resp(w)

	tp := r.Header.Get("Content-Type")
	if strings.Count(tp, "json") == 1 { //json格式
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			glog.V(0).Infof("ReadAll err:%v", err)
			result.Code = "401"
			result.Msg = "读取参数有误"
			return
		}
		body_str = string(body)
		glog.V(3).Infof("json格式请求报文:%s\n", body_str)
		//err = json.Unmarshal(body, &cm)
		//if err != nil {
		//	result.Code = "401"
		//	result.Msg = fmt.Sprintf("参数有误,%v", err)
		//	return
		//}
	}

	gCH_cm <- body_str

	result.Msg = "提交成功"
	result.Code = "200"
}

func do_update_cm() {
	//cmdburl := conf.GetSerConf().CmdbAddress_value
	for {
		//测试cmdb接口是否可达

		//从通道处读取数据
		v, ok := <-gCH_cm
		if !ok {
			glog.V(1).Infof("数据读取完毕.")
			break
		}
		glog.V(3).Infof("处理服务从通道获取数据:[%v]\n", v)

		//post 调用cmdb提供的接口
		//r, err := pub.PostJson(cmdburl, v)
		//glog.V(3).Infof("处理结果%s\n", r)
		//if err != nil {
		//	glog.V(0).Infof("updatecm failed,err:%v\n", err)
		//}
	}
}

func PutSerConf(client *etcd.EtcdClient) (err error) {
	svrconf := conf.GetSerConf()
	if err = client.PutKey(svrconf.ServerAddress_key, svrconf.ServerAddress_value); err != nil {
		glog.V(0).Infof("putkey err:%v", err)
		return
	}
	if err = client.PutKey(svrconf.CmdbAddress_key, svrconf.CmdbAddress_value); err != nil {
		glog.V(0).Infof("putkey err:%v", err)
		return
	}
	if err = client.PutKey(svrconf.ECAddress_key, svrconf.ECAddress_value); err != nil {
		glog.V(0).Infof("putkey err:%v", err)
		return
	}
	if err = client.PutKey(svrconf.CliTtl_key, svrconf.CliTtl_value); err != nil {
		glog.V(0).Infof("putkey err:%v", err)
		return
	}
	if err = client.PutKey(svrconf.SoftCheck_key, svrconf.SoftCheckList); err != nil {
		glog.V(0).Infof("putkey err:%v", err)
		return
	}
	return nil
}

func FinishHandle() {
	go SaveCliRegInfo()
	go httpServerCheck()
	go do_update_cm()
}

//检查服务是否启动完成
func httpServerCheck() {
	for {
		glog.V(0).Info("checking svr started yet ?")
		time.Sleep(time.Second)
		resp, _ := pub.Get("http://localhost:7788/help")
		if len(strings.TrimSpace(resp)) == 0 {
			continue
		} else {
			glog.V(0).Info("svr has start success!")
			break
		}
	}
}

func SaveCliRegInfo() {
	for {
		time.Sleep(time.Minute)
		if !gDisCli.NeedFlash {
			continue
		}
		cliinfolist := make([]etcd.CliRegInfo, 0)
		for _, v := range gDisCli.ServerList {
			cliinfolist = append(cliinfolist, v)
		}
		flashCliRegInfoToFile(&cliinfolist)
		gDisCli.NeedFlash = false
	}
}

func flashCliRegInfoToFile(reginfo *[]etcd.CliRegInfo) {
	buf, err := json.MarshalIndent(reginfo, "", "    ")
	if err != nil {
		glog.V(0).Infof("json.Marshal err: [%v]", err)
		return
	}
	resultfile := conf.GetSerConf().CliRegInfo
	syscall.Umask(0000)
	err = ioutil.WriteFile(resultfile, buf, 0600)
	if err != nil {
		glog.V(0).Infof("ioutil.WriteFile failure, err=[%v]\n", err)
	}
}
