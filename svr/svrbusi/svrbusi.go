package svrbusi

import (
	"fmt"
	"github.com/golang/glog"
	"io/ioutil"
	"net/http"
	"os"
	"shagt/conf"
	"shagt/etcd"
	"shagt/pub"
	"strings"
	"time"
)

var gDisCli *etcd.ClientDis

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
	cliList := getServerList(strings.TrimSpace(host))
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

	r.ParseForm()
	stmp, ok := r.Form["host"]
	if !ok {
		result.Code = "401"
		result.Msg = "参数有误,请指定服务器。"
		result.Resp(w)
	} else {
		host = stmp[0]
		cliList := getServerList(strings.TrimSpace(host))
		if len(cliList) != 1 {
			result.Code = "401"
			result.Msg = fmt.Sprintf("参数有误,请指定服务器,查询服务器列表:%v", cliList)
			result.Resp(w)
		} else {
			cli := strings.Split(cliList[0], ",") //hostname,ip,pid,ver
			url := fmt.Sprintf("http://%s:7790/monitor", cli[1])
			mon,_ := pub.Get(url)
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
			result.Msg = fmt.Sprintf("open file err,%v",err)
			glog.V(0).Infof("open file err,%v",err)
			result.Resp(w)
			return
		}
		defer file.Close()
		content, err := ioutil.ReadAll(file)
		if err != nil {
			result.Code = "401"
			result.Msg = fmt.Sprintf("Read file err,%v",err)
			glog.V(0).Infof("Read file err,%v",err)
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
		if len(host) == 0 || strings.Contains(v, host) {
			list = append(list, v)
		}
	}
	return
}
func Svr_handler(w http.ResponseWriter, r *http.Request) {
	var result pub.Resp

	defer result.Resp(w)

	tp := r.Header.Get("Content-Type")
	if strings.Contains(tp, "json") { //json格式
		result.Code = "401"
		result.Msg = "参数有误,暂不支持json格式"
	} else { //POST 请求，form格式
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			glog.V(0).Infof("ReadAll err:%v", err)
			result.Code = "401"
			result.Msg = "读取参数有误"
			return
		}
		body_str := string(body)
		glog.V(3).Infof("请求报文:%s\n", body_str) //host=192.168.0.111&action=reboot
		result.Code = "200"
	}

	//if err != nil {
	//	result.Msg = fmt.Sprintf("提交失败,%v", err)
	//} else {
	//	result.Msg = "提交成功"
	//}

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
