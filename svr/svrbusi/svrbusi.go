package svrbusi

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"shagt/conf"
	"shagt/etcd"
	"shagt/pub"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var gDisCli *etcd.ClientDis

var gCH_cm = make(chan string, 512)                 //客户端配置信息队列
var gCH_wantoec = make(chan WarnInfo, 512)          //预警信息队列
var gCH_OriginalId = make(chan string, 16)          //从该队列获取id
var gCH_OriginalId_notify = make(chan struct{}, 16) //通知生成id

//用户查询服务器端已注册的服务器信息
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
			url := fmt.Sprintf("http://%s:17789/monitor", cli[1])
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
			url = fmt.Sprintf("http://%s:17789/op2", cli[1])
			data = fmt.Sprintf("action=%s", action)
		} else if action == "update" {
			url = fmt.Sprintf("http://%s:17789/op2", cli[1])
			data = fmt.Sprintf("action=%s&ver=%s", action, ver)
		} else {
			url = fmt.Sprintf("http://%s:17789/op", cli[1])
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
			result.Resp(w)
			return
		}
		body_str = string(body)
		glog.V(3).Infof("update-cm,json格式请求报文:%s\n", body_str)
	} else {
		result.Code = "401"
		result.Msg = "暂只支持json格式"
		result.Resp(w)
		return
	}

	gCH_cm <- body_str

	result.Msg = "提交成功"
	result.Code = "200"
	result.Resp(w)
}

func do_update_cm() {
	cmdburl := conf.GetSerConf().CmdbAddress_value
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
		r, err := pub.PostJson(cmdburl, v)
		glog.V(3).Infof("处理结果%s\n", r)
		if err != nil {
			glog.V(0).Infof("updatecm failed,err:%v\n", err)
		}
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
		time.Sleep(time.Second * 60)
	}
}

func FinishHandle() {
	go loadEmpFile()
	go SaveCliRegInfo()
	go httpServerCheck()
	go do_update_cm()
	go genOriginalid()
	go genOriginalid2()
	go doSendtoEC()
	go signalHander()

	time.Sleep(time.Second)
	glog.Flush()
}

//检查服务是否启动完成
func httpServerCheck() {
	for {
		glog.V(0).Info("checking svr started yet ?")
		time.Sleep(time.Second)
		resp, _ := pub.Get("http://127.0.0.1:17788/help")
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

type WarnInfo struct {
	EcType           string `json:"ectype"`
	Id_original      string `json:"id_original"` //如为空则生成一个序号,类似：yyyymmddhhmmss001
	Ip               string `json:"ip"`
	Hostname         string `json:"hostname"`
	Source           string `json:"source"`
	Category         string `json:"category"`
	Object_class     string `json:"object_class"`
	Object           string `json:"object"`
	Instance         string `json:"instance"`
	Parameter        string `json:"parameter"`
	Parameter_value  string `json:"parameter_value"`
	Severity         string `json:"severity"` //1：严重、2：主要、3：次要、4：预警
	Title            string `json:"title"`
	Summary          string `json:"summary"`
	First_occurrence string `json:"first_occurrence"`
	Status           string `json:"status"` //1打开,2自动关闭，告警解除后请求端传入；3运维人员手工关闭；4第三方关闭；
	ShowTimes        string `json:"showtimes"`
	NoticeWay        string `json:"noticeway"`
	NoticeEmpNo      string `json:"noticeempno"`
	NoticeEmpNo1     string `json:"noticeempno1"`
	NoticeEmpNo2     string `json:"noticeempno2"`
	NoticeEmpNo3     string `json:"noticeempno3"`
	NoticeEmpNo4     string `json:"noticeempno4"`
	Filed1           string `json:"filed1"`
	Filed2           string `json:"filed2"`
	Filed3           string `json:"filed3"`
}

//1、解析接收到的报文，给结构体赋值
//	json
//	form
//2、预处理，并发送到队列
//3、从队列取值并发送
func WarnToEC(w http.ResponseWriter, r *http.Request) {
	var result pub.Resp
	var warninfo WarnInfo

	glog.V(0).Infof("warntoec request :%s", r.URL.String())

	if r.Method != "POST" {
		result.Code = "401"
		result.Msg = "只允许POST请求"
		result.Resp(w)
		return
	}

	tp := r.Header.Get("Content-Type")
	if strings.Count(tp, "json") == 1 { //json格式
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			glog.V(0).Infof("ReadAll err:%v", err)
			result.Code = "401"
			result.Msg = "读取参数有误"
			result.Resp(w)
			return
		}
		body_str := string(body)
		glog.V(3).Infof("json格式请求报文:%s\n", body_str)

		err = json.Unmarshal(body, &warninfo)
		if err != nil {
			result.Code = "401"
			result.Msg = "参数有误,Unmarshal err," + err.Error()
			result.Resp(w)
			return
		}
	} else { //form格式
		err := r.ParseForm()
		if err != nil {
			result.Code = "401"
			result.Msg = "参数有误"
			result.Resp(w)
			return
		}
		//必须输入ip、Summary
		arr_ip, ipok := r.Form["ip"]
		arr_summary, sumok := r.Form["summary"]
		if !ipok || !sumok {
			result.Code = "401"
			result.Msg = "form格式数据获取参数有误"
			result.Resp(w)
			return
		}
		warninfo.Ip = arr_ip[0]
		warninfo.Summary = arr_summary[0]
		//可选输入id_original,severity,status
		arr_id, idok := r.Form["id_original"]
		arr_severity, secok := r.Form["severity"]
		arr_status, statok := r.Form["status"]
		arr_ectype, ectypeok := r.Form["ectype"]
		if idok {
			warninfo.Id_original = arr_id[0]
		}
		if secok {
			warninfo.Severity = arr_severity[0]
		}
		if statok {
			warninfo.Status = arr_status[0]
		}
		if ectypeok {
			warninfo.Status = arr_ectype[0]
		}
	}

	glog.V(3).Infof("before prepare....\n")
	prepareWarninfo(&warninfo)
	glog.V(3).Infof("after prepare....\n")

	gCH_wantoec <- warninfo

	result.Msg = "提交成功"
	result.Code = "200"
	result.Resp(w)
}

//根据请求生成original_id
//未指定事件类型，无需保留状态的id
func genOriginalid() {
	var lasttm, now, id string
	var num int8
	for {
		//从通道处读取数据
		_, ok := <-gCH_OriginalId_notify
		if !ok {
			glog.V(1).Infof("数据读取完毕.")
			break
		}
		glog.V(3).Infof("------请求生成originalid\n")
		now = pub.GetTimeStr6()
		if strings.Compare(lasttm, now) == 0 {
			num++
		} else {
			num = 1
		}
		id = fmt.Sprintf("%s%03d", now, num)
		lasttm = now
		gCH_OriginalId <- id
		glog.V(3).Infof("-----生成------>id:%s", id)
	}
}

const NUM = 10 //获取事件id，设置10个并发队列
var mutex, mutex2 sync.Mutex
var syncNum int8

type originalidreq struct {
	key    string
	status string
	chno   int8
}

var gCH_ecidreq = make(chan originalidreq, 16) //请求队列
var arr_ecid_resp [NUM](chan string)           //数组中存放请求返回队列，从该队列获取id
var arr_req []byte                             //队列位图,开始000000000

//根据请求队列，串行处理
func genOriginalid2() {
	syncNum = NUM
	for i := 0; i < NUM; i++ {
		arr_req = append(arr_req, '0')
		arr_ecid_resp[i] = make(chan string, 1)
		glog.V(1).Infof("%2d--------------->%p", i, arr_ecid_resp[i])
	}
	//glog.V(1).Infof("%s", string(arr_req))

	//etcd 连接，设置，监听key变化读取客户端注册清单
	etcdcli, err := etcd.NewEtcdClient([]string{conf.GetSerConf().EtcdAddress})
	if err != nil {
		glog.V(0).Infof("etcd.NewEtcdClient err,%v", err)
		return
	}

	for {
		//从通道处读取数据
		v, ok := <-gCH_ecidreq
		if !ok {
			glog.V(1).Infof("数据读取完毕.")
			break
		}
		glog.V(3).Infof("------请求根据ectype生成originalid,%v\n", v)
		glog.V(6).Infof("--------------->%p", arr_ecid_resp[v.chno])
		strings.TrimSpace(v.key)
		k := fmt.Sprintf("/svr/ectype/%s", v.key)
		kv, err := etcdcli.GetKey(k)
		if err != nil {
			glog.V(0).Infof("etcdcli.GetKey err,%v\n", err)
			continue
		}

		var id string
		var num int
		if len(*(kv)) == 0 {
			num = 1
		} else if len(*(kv)) == 1 {
			n, err := strconv.Atoi((*kv)[k])
			if err != nil {
				num = 1
			} else {
				num = n
			}
		} else if len(*(kv)) > 1 {
			num = -1
			glog.V(0).Infof("------上送的ectype非法,%v\n", v)
		}
		if num >= 0 {
			id = fmt.Sprintf("%s-%d", v.key, num)
			if strings.Compare(v.status, "2") == 0 {
				num++
				etcdcli.PutKey(k, fmt.Sprintf("%d", num))
			}
		} else {
			id = fmt.Sprintf("%s-%s", v.key, pub.GetTimeStr6)
		}

		arr_ecid_resp[v.chno] <- id
		glog.V(3).Infof("-----生成------>id:%s", id)
	}
}

func getOriginalid(warninfo *WarnInfo) {
	//未设置ectype，通知类
	if len(warninfo.EcType) == 0 {
		gCH_OriginalId_notify <- struct{}{}
		select {
		case id := <-gCH_OriginalId:
			warninfo.Id_original = id
		case <-time.After(2 * time.Second):
			warninfo.Id_original = fmt.Sprintf("%s001", pub.GetTimeStr6())
			glog.V(1).Infof("获取新的事件id失败,timeout.")
		}
		return
	}

	//根据ectype从etcd获取事件id
	var chNo = -1
	var retry = 2 //首次失败后重试3次
loop:
	mutex.Lock()
	if syncNum > 0 {
		for i := 0; i < NUM; i++ {
			if arr_req[i] == '0' {
				arr_req[i] = '1'
				chNo = i
				syncNum--
				break
			}
		}
	}
	mutex.Unlock()

	glog.V(3).Infof("chNo=%d", chNo)
	if chNo < 0 {
		if retry > 0 {
			retry--
			time.Sleep(time.Millisecond * 100)
			goto loop
		}
		glog.V(1).Infof("获取队列编号失败.")
		return
	}

	reqdata := originalidreq{
		key:    warninfo.EcType,
		status: warninfo.Status,
		chno:   int8(chNo),
	}
	glog.V(6).Infof("--------------->%p", arr_ecid_resp[chNo])
	gCH_ecidreq <- reqdata
	select {
	case id := <-arr_ecid_resp[chNo]:
		warninfo.Id_original = id
		mutex2.Lock()
		arr_req[chNo] = '0'
		syncNum++
		mutex2.Unlock()
	case <-time.After(2 * time.Second):
		glog.V(1).Infof("获取新的事件id失败,timeout.")
	}
}

func prepareWarninfo(warninfo *WarnInfo) {
	if len(warninfo.Status) == 0 {
		warninfo.Status = "1" //打开事件,根据ectype获取id需要status，故现行检查
	}

	if len(warninfo.Id_original) == 0 {
		getOriginalid(warninfo)
		glog.V(0).Infof("新的事件id:%s", warninfo.Id_original)
	}

	//设置默认值
	if len(warninfo.First_occurrence) == 0 {
		warninfo.First_occurrence = pub.GetTimeStr1()
	}
	if len(warninfo.Category) == 0 {
		warninfo.Category = "web"
	}
	if len(warninfo.Title) == 0 {
		warninfo.Title = "预警"
	}
	if len(warninfo.Severity) == 0 {
		warninfo.Severity = "4" //一般预警
	}
	if len(warninfo.ShowTimes) == 0 {
		if warninfo.Severity == "4" {
			warninfo.ShowTimes = "900"
		} else {
			warninfo.ShowTimes = "1800"
		}
	}

	if len(warninfo.Source) == 0 {
		warninfo.Status = "RT_agent" //代理渠道
	}
	if len(warninfo.NoticeWay) == 0 {
		warninfo.NoticeWay = "1"
	}

	//如果送的是员工号，转换成姓名
	empinfo1 := warninfo.NoticeEmpNo1
	empinfo2 := ""
	emp_arr := strings.Split(empinfo1, "|")
	for _, v := range emp_arr {
		if []byte(v)[0] >= '0' && []byte(v)[0] <= '9' {
			name, ok := gEmpInfo[v]
			if !ok {
				glog.V(0).Infof("未设置员工信息,%s", v)
				continue
			}
			empinfo2 = empinfo2 + name + "|"
		} else {
			empinfo2 = empinfo2 + v + "|"
		}
	}
	warninfo.NoticeEmpNo = empinfo2
}

func doSendtoEC() {
	var flag int8
	ecurl := conf.GetSerConf().ECAddress_value
	if len(strings.TrimSpace(ecurl)) == 0 {
		glog.V(0).Infof("未设置事件中心地址\n")
		flag = 1
	}

	for {
		//测试ec接口是否可达

		//从通道处读取数据
		v, ok := <-gCH_wantoec
		if !ok {
			glog.V(1).Infof("数据读取完毕.")
			break
		}
		glog.V(3).Infof("处理服务从通道获取数据:[%v]\n", v)

		byteWarninfo, err := json.Marshal(v)
		if err != nil {
			glog.V(0).Infof("json.Marshal err:%v\n", err)
			continue
		}
		glog.V(4).Infof("warninfo:%s\n", string(byteWarninfo))
		//post 调用cmdb提供的接口
		if flag == 1 {
			continue
		}
		r, err := pub.PostJson(ecurl, string(byteWarninfo))
		glog.V(3).Infof("事件处理结果%s\n", r)
		if err != nil {
			glog.V(0).Infof("向事件中心发送事件,err:%v\n", err)
		}
	}
}

var gCH_loadEmpFile = make(chan struct{}, 1)
var gEmpInfo = make(map[string]string) //收到预警信息后根据员工后获取姓名

func loadEmpFile() {
	var flag int8
	empfile := conf.GetSerConf().EmpFile
	glog.V(4).Infof("员工信息文件EmpFile:%s", empfile)
	if len(strings.TrimSpace(empfile)) == 0 {
		glog.V(0).Infof("未设置EmpFile\n")
		flag = 1
	}
	gCH_loadEmpFile <- struct{}{}

	for {
		//从通道处读取数据
		_, ok := <-gCH_loadEmpFile
		if !ok {
			glog.V(1).Infof("数据读取完毕.")
			break
		}
		if flag == 1 {
			glog.V(0).Infof("未设置EmpFile\n")
			continue
		}

		glog.V(3).Infof("开始加载员工信息文件:[%s]\n", empfile)
		empData, err := ioutil.ReadFile(empfile)
		if err != nil {
			glog.V(0).Infof("ioutil.ReadFile err,%v", err)
			continue
		}
		if err = json.Unmarshal(empData, &gEmpInfo); err != nil {
			glog.V(0).Infof("json.Unmarshal err,%v", err)
			continue
		}
	}
}

func signalHander() {
	ch_sig := make(chan os.Signal)
	signal.Notify(ch_sig, syscall.SIGUSR1)
	for {
		v := <-ch_sig
		if v == syscall.SIGUSR1 {
			glog.V(0).Info("收到自定义中断信号SIGUSR1，执行开始加载员工信息文件操作。")
			gCH_loadEmpFile <- struct{}{}
		}
	}
}
