package ps

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/net"
	"github.com/shirou/gopsutil/process"
	"io/ioutil"
	"shagt/comm"
	"shagt/conf"
	"shagt/pub"
	"strings"
	"syscall"
	"time"
)

//实时监控数据
type MonServer struct {
	Host      HostInfo
	CPU       CPUInfo
	Mem       MemInfo
	Swap      SwapInfo
	Interface []InterfaceInfo
	Sys       SysInfo
	//Nfs       []NfsInfo
	Fs      []Partition
	Process []ProcessInfo
	Soft    []SoftInfo
}

//配置管理数据
type CMStru struct {
	Host      Host
	Cpu       CPU
	Mem       Mem
	Swap      Swap
	Interface []Interface
	Sys       SysInfo
	//NFS       []NfsInfo
	Fs   []Partition
	Soft []SoftInfo
}

type HostInfo struct {
	Hostname        string
	OS              string
	PlatformVersion string
	KernelVersion   string
	BootTime        uint64
	Uptime          uint64
}
type Host struct {
	Hostname        string
	OS              string
	PlatformVersion string
	KernelVersion   string
}

type CPUInfo struct {
	CoreNum     int
	UsedPercent float64
}
type CPU struct {
	CoreNum int
}

type MemInfo struct {
	Total       int //M
	UsedPercent float64
}
type Mem struct {
	Total int //M
}
type SwapInfo struct {
	Total       int
	UsedPercent float64
}
type Swap struct {
	Total int
}

type InterfaceInfo struct {
	Name  string
	Addrs []string
	Mac   string
}

type Interface struct { //多条记录
	Name  string
	Addrs string //单网卡单ip
	Mac   string
}

type SysInfo struct {
	Dns        string
	HasNtp     bool
	HasIptable bool
}

type NfsInfo struct {
	NfsSvr     string
	NfsName    string
	MountPoint string
}
type Partition struct {
	Device     string
	Mountpoint string
	Fstype     string
	Opts       string
}

type ProcessInfo struct {
	Port       uint32
	Username   string
	Pid        int32
	CreateTime int64
	Cmdline    string
	Note       string
}

type SoftInfo struct {
	SoftName string
	CmdLine  string
	Ver      string
	Vershow  string
}

type CmUp struct {
	Key   string
	Value string
	Op    string
}

//var gSoftlist_last *[]SoftInfo
var gCM_last *CMStru

func GetMachineInfo() *MonServer {

	//d, _ := disk.Usage("/")
	//nv, _ := net.IOCounters(true)

	hinfo, _ := host.Info()

	cpu_num, _ := cpu.Counts(true)
	cpu_per, _ := cpu.Percent(time.Second, false)

	memo, _ := mem.VirtualMemory() //total/UsedPercent
	swap, _ := mem.SwapMemory()    //total/UsedPercent

	inet, _ := net.Interfaces()
	d, _ := disk.Partitions(true)

	//proc, _ := process.Processes()

	ss := new(MonServer)

	ss.Host.Hostname = hinfo.Hostname
	ss.Host.OS = hinfo.OS
	ss.Host.PlatformVersion = hinfo.PlatformVersion
	ss.Host.KernelVersion = hinfo.KernelVersion
	ss.Host.Uptime = hinfo.Uptime
	ss.Host.BootTime = hinfo.BootTime

	ss.CPU.CoreNum = cpu_num
	ss.CPU.UsedPercent = func(p []float64) float64 {
		sum := 0.0
		n := 0
		for _, v := range p {
			sum += v
			n++
		}
		return pub.MathTrunc(sum/float64(n), 2)
	}(cpu_per)

	ss.Mem.Total = int(memo.Total / 1024 / 1024)
	ss.Mem.UsedPercent = pub.MathTrunc(memo.UsedPercent, 2)
	ss.Swap.Total = int(swap.Total / 1024 / 1024)
	ss.Swap.UsedPercent = pub.MathTrunc(swap.UsedPercent, 2)

	ss.Interface = make([]InterfaceInfo, 0)
	for _, v := range inet {
		if v.Name == "lo0" {
			continue
		}
		n := InterfaceInfo{
			Name:  v.Name,
			Addrs: make([]string, 0),
			Mac:   v.HardwareAddr,
		}
		for _, vv := range v.Addrs {
			n.Addrs = append(n.Addrs, vv.Addr)
		}
		ss.Interface = append(ss.Interface, n)
	}

	ss.Fs = make([]Partition, 0)
	for _, v := range d {
		pt := Partition{
			Device:     v.Device,
			Mountpoint: v.Mountpoint,
			Fstype:     v.Fstype,
			Opts:       v.Opts,
		}
		ss.Fs = append(ss.Fs, pt)
	}

	ss.Process = make([]ProcessInfo, 0)
	//proc_map := make(map[int32]*process.Process)
	//for _, v := range proc {
	//	proc_map[v.Pid] = v
	//}
	pidlist_check := make([]int32, 0)
	conn, _ := net.Connections("all")
	for _, v2 := range conn {
		if v2.Status == "LISTEN" {
			p := process.Process{
				Pid: v2.Pid,
			}
			ctm, _ := p.CreateTime()
			cmd, _ := p.Cmdline()
			proc_entry := ProcessInfo{
				Port:       v2.Laddr.Port,
				Username:   "",
				Pid:        v2.Pid,
				CreateTime: ctm,
				Cmdline:    cmd,
				Note:       "",
			}
			ss.Process = append(ss.Process, proc_entry)
			pidlist_check = append(pidlist_check, v2.Pid)
		}
	}

	//softWareList := []string{"tomcat", "webshpere", "redis", "zookeeper", "dubbo", "qq", "Goland"}
	soft := comm.G_ReadFromServerConf.SoftCheck
	softWareList := make([]string, 0)
	err := json.Unmarshal([]byte(soft), &softWareList)
	if err != nil {
		glog.V(0).Infof("获取软件列表错误,json.Unmarshal err,%v", err)
	}
	glog.V(3).Infof("softWareList:%v", softWareList)
	softAll := make(map[string]SoftInfo)
	for _, v := range pidlist_check {
		p := process.Process{
			Pid: v,
		}
		cmd, _ := p.Cmdline()
		for _, soft := range softWareList {
			if strings.Contains(strings.ToUpper(cmd), strings.ToUpper(soft)) {
				s := SoftInfo{
					SoftName: soft,
					CmdLine:  cmd,
					Ver:      "",
				}
				softAll[soft+cmd] = s
				break
			}
		}
	}
	ss.Soft = *getSoftWareVer(&softAll)

	//检查发送给配置库的信息是否有更新
	ss.CheckCM()

	return ss

	//b, err := json.MarshalIndent(ss, "", "    ")
	//if err != nil {
	//	return ""
	//} else {
	//	return string(b)
	//}
}

func getSoftWareVer(softmap_now *map[string]SoftInfo) *[]SoftInfo {
	softList_now := make([]SoftInfo, 0)
	softlist_last := readSoftCheckFile()
	softmap_last := make(map[string]SoftInfo, 0)
	for _, v := range *softlist_last {
		softmap_last[v.SoftName+v.CmdLine] = v
	}
	for k, v := range *softmap_now {
		last, ok := softmap_last[k]
		if !ok { //首次执行，或新增
			softList_now = append(softList_now, v)
			continue
		}
		v.Vershow = last.Vershow
		if len(last.Ver) > 0 { //直接指定版本
			v.Ver = last.Ver
			softList_now = append(softList_now, v)
			continue
		}
		if len(last.Vershow) > 0 { //通过执行该命令获取版本
			glog.V(3).Infof("通过脚本命令获取版本,%s", last.Vershow)
			ver, err := pub.ExecOSCmd(last.Vershow)
			if err != nil {
				glog.V(3).Infof("脚本命令获取版本失败,%v", err)
			} else {
				v.Ver = ver
			}
			softList_now = append(softList_now, v)
			continue
		}
		softList_now = append(softList_now, v)
	}

	flashSoftCheckFile(&softList_now)

	return &softList_now
}

func readSoftCheckFile() *[]SoftInfo {
	filepath := conf.GetCliConf().SoftWareCheckPath
	softlist_his := make([]SoftInfo, 0)
	if ok, err := pub.IsFile(filepath); !ok {
		glog.V(0).Infof("SoftWareCheckPath: %s, err: [%v]", filepath, err)
		return &softlist_his
	}
	buf, err := ioutil.ReadFile(filepath)
	if err != nil {
		glog.V(0).Infof("read file: %s, err: [%v]", filepath, err)
		return &softlist_his
	}
	err = json.Unmarshal(buf, &softlist_his)
	if err != nil {
		glog.V(0).Infof("json.Unmarshal err: [%v]", err)
		return nil
	}

	return &softlist_his
}

func flashSoftCheckFile(softlist *[]SoftInfo) {
	buf, err := json.MarshalIndent(softlist, "", "    ")
	if err != nil {
		glog.V(0).Infof("json.Marshal err: [%v]", err)
		return
	}
	resultfile := conf.GetCliConf().SoftWareCheckPath
	syscall.Umask(0000)
	err = ioutil.WriteFile(resultfile, buf, 0600)
	if err != nil {
		glog.V(0).Infof("ioutil.WriteFile failure, err=[%v]\n", err)
	}
}

func (ss *MonServer) CheckCM() error {
	//生成最新配置信息
	cm_now := ss.getCM()
	if gCM_last == nil {
		cm_file, err := readCfgManageFile()
		if err == nil {
			gCM_last = cm_file
		}
	}
	//生成提交给配置库的信息（变动信息）
	cmup := comprareCM(cm_now, gCM_last)
	glog.V(3).Infof("配置变动信息:%v", cmup)

	gCM_last = cm_now
	gCM_last.flashCfgManageFile()

	return nil
}

func readCfgManageFile() (*CMStru, error) {
	filepath := conf.GetCliConf().CfgManageFilePath
	cm_his := new(CMStru)
	if ok, err := pub.IsFile(filepath); !ok {
		glog.V(0).Infof("CfgManageFilePath: %s, err: [%v]", filepath, err)
		return cm_his, err
	}
	buf, err := ioutil.ReadFile(filepath)
	if err != nil {
		glog.V(0).Infof("read file: %s, err: [%v]", filepath, err)
		return cm_his, err
	}
	err = json.Unmarshal(buf, cm_his)
	if err != nil {
		glog.V(0).Infof("json.Unmarshal err: [%v]", err)
		return cm_his, err
	}
	return cm_his, nil
}

func (cm *CMStru) flashCfgManageFile() {
	buf, err := json.MarshalIndent(gCM_last, "", "    ")
	if err != nil {
		glog.V(0).Infof("json.Marshal err: [%v]", err)
		return
	}
	syscall.Umask(0000)
	filepath := conf.GetCliConf().CfgManageFilePath
	err = ioutil.WriteFile(filepath, buf, 0600)
	if err != nil {
		glog.V(0).Infof("ioutil.WriteFile failure, err=[%v]\n", err)
	}
}

func (ss *MonServer) getCM() *CMStru {
	var cm *CMStru
	if ss == nil {
		return cm
	}
	cm = new(CMStru)
	cm.Host.Hostname = ss.Host.Hostname
	cm.Host.OS = ss.Host.OS
	cm.Host.PlatformVersion = ss.Host.PlatformVersion
	cm.Host.KernelVersion = ss.Host.KernelVersion
	cm.Cpu.CoreNum = ss.CPU.CoreNum
	cm.Mem.Total = ss.Mem.Total
	cm.Swap.Total = ss.Swap.Total
	cm.Interface = make([]Interface, 0)
	for _, v := range ss.Interface {
		var addr string
		for _, v2 := range v.Addrs {
			addr = addr + v2
		}
		inet := Interface{
			Name:  v.Name,
			Addrs: addr,
			Mac:   v.Mac,
		}
		cm.Interface = append(cm.Interface, inet)
	}
	cm.Fs = make([]Partition, 0)
	for _, v := range ss.Fs {
		cm.Fs = append(cm.Fs, v)
	}
	cm.Sys.Dns = ss.Sys.Dns
	cm.Sys.HasNtp = ss.Sys.HasNtp
	cm.Sys.HasIptable = ss.Sys.HasIptable
	cm.Soft = make([]SoftInfo, 0)
	for _, v := range ss.Soft {
		soft := SoftInfo{
			SoftName: v.SoftName,
			CmdLine:  v.CmdLine,
			Ver:      v.Ver,
			Vershow:  v.Vershow,
		}
		cm.Soft = append(cm.Soft, soft)
	}

	return cm
}

func comprareCM(now, last *CMStru) *[]CmUp {
	m_now := now.generateKV()
	m_last := last.generateKV()
	map_union := make(map[string]string, 0)
	cmupList := make([]CmUp, 0)
	for k, v := range *m_now {
		map_union[k] = v
	}
	for k, v := range *m_last {
		map_union[k] = v
	}
	for k, _ := range map_union {
		v_now, ok_now := (*m_now)[k]
		v_last, ok_last := (*m_last)[k]
		var note string
		if ok_now && ok_last && v_now == v_last {
			continue
		}
		if ok_now && ok_last && v_now != v_last { //up
			note = "update"
		}
		if !ok_now && ok_last { //del
			note = "del"
		}
		if ok_now && !ok_last { //add
			note = "add"
		}
		cmup := CmUp{
			Key:   k,
			Value: v_now,
			Op:    note,
		}
		cmupList = append(cmupList, cmup)
	}
	return &cmupList
}

func (cm *CMStru) generateKV() *map[string]string {
	m := make(map[string]string, 0)
	if cm == nil {
		return &m
	}
	if len(cm.Host.Hostname) > 0 {
		m["Host.Hostname"] = cm.Host.Hostname
	}
	if len(cm.Host.OS) > 0 {
		m["Host.OS"] = cm.Host.OS
	}
	if len(cm.Host.PlatformVersion) > 0 {
		m["Host.PlatformVersion"] = cm.Host.PlatformVersion
	}
	if len(cm.Host.KernelVersion) > 0 {
		m["Host.KernelVersion"] = cm.Host.KernelVersion
	}

	if cm.Cpu.CoreNum > 0 {
		m["Cpu.CoreNum"] = fmt.Sprintf("%d", cm.Cpu.CoreNum)
	}
	if cm.Mem.Total > 0 {
		m["Mem.Total"] = fmt.Sprintf("%d", cm.Mem.Total)
	}
	if cm.Swap.Total > 0 {
		m["Swap.Total"] = fmt.Sprintf("%d", cm.Swap.Total)
	}

	for _, i := range cm.Interface {
		m["Interface."+i.Name] = fmt.Sprintf("%s|%s", i.Addrs, i.Mac)
	}

	if len(cm.Sys.Dns) > 0 {
		m["Sys.Dns"] = cm.Sys.Dns
	}
	m["Sys.HasNtp"] = fmt.Sprintf("%v", cm.Sys.HasNtp)
	m["Sys.HasIptable"] = fmt.Sprintf("%v", cm.Sys.HasIptable)

	for _, v := range cm.Fs {
		m["FS."+v.Mountpoint] = fmt.Sprintf("%s|%s|%s|%s", v.Device, v.Mountpoint, v.Fstype, v.Opts)
	}

	for _, v := range cm.Soft {
		m["Soft."+v.SoftName+"."+v.CmdLine] = fmt.Sprintf("%s|%s|%s|%s", v.SoftName, v.CmdLine, v.Ver, v.Vershow)
	}
	return &m
}
