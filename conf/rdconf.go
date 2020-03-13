package conf

import (
	"github.com/golang/glog"
	"gopkg.in/ini.v1"
	"path/filepath"
	"shagt/comm"
	"strings"
)

type SerConf struct {
	EtcdAddress         string
	ServerAddress_key   string
	ServerAddress_value string
	CmdbAddress_key     string
	CmdbAddress_value   string
	ECAddress_key       string
	ECAddress_value     string
	CliTtl_key          string
	CliTtl_value        string
	SoftCheck_key       string
	SoftCheckList       string
	CliSoftPath         string
}

type CliConf struct {
	LocalHostName     string
	LocalHostIp       string
	EtcdAddress       string
	ServerAddress_key string
	CmdbAddress_key   string
	ECAddress_key     string
	CliTtl_key        string
	SoftCheck_key     string
	CliLogMonPath     string
	DrCheckFilePath   string
	SoftWareCheckPath string
	CfgManageFilePath string
	LocalNetTarget    string
}

var gSerConf = &SerConf{}
var gCliConf = &CliConf{}

func init() {

}
func InitSerConf(cfgfile string) (err error) {
	//cfg, _ := ini.Load("/Users/shenjq/go/src/demo/idm/conf/conf.ini")
	//err = cfg.MapTo(gIni)
	//err = ini.MapTo(gSerConf, "/Users/shenjq/go/src/demo/shagt/conf/server-config.ini")
	err = ini.MapTo(gSerConf, cfgfile)
	if err != nil {
		glog.V(0).Infof("initconf err,%v", err)
		return
	}
	var workpath string
	workpath, err = comm.GetWorkPath()
	if err != nil {
		glog.V(0).Infof("GetCurrentPath err,%v", err)
		return
	}
	if !filepath.IsAbs(gSerConf.CliSoftPath) {
		gSerConf.CliSoftPath = workpath + gSerConf.CliSoftPath
	}
	glog.V(3).Infof("ser-conf=%v", gSerConf)
	return
}

func InitCliConf(cfgfile string) (err error) {
	glog.V(0).Infof("** config file:%s", cfgfile)
	err = ini.MapTo(gCliConf, cfgfile)
	if err != nil {
		glog.V(0).Infof("initconf err,%v", err)
		return
	}
	if strings.TrimSpace(gCliConf.LocalHostName) == "" { //获取本机hostname
		gCliConf.LocalHostName = comm.GetHostName()
	}
	if strings.TrimSpace(gCliConf.LocalHostIp) == "" { //获取本机ip
		gCliConf.LocalHostIp = comm.GetMgrIP(gCliConf.LocalNetTarget)
	}
	if !filepath.IsAbs(gCliConf.CliLogMonPath) {
		gCliConf.CliLogMonPath = comm.G_CliInfo.WorkPath + gCliConf.CliLogMonPath
	}
	if !filepath.IsAbs(gCliConf.DrCheckFilePath) {
		gCliConf.DrCheckFilePath = comm.G_CliInfo.WorkPath + gCliConf.DrCheckFilePath
	}
	if !filepath.IsAbs(gCliConf.SoftWareCheckPath) {
		gCliConf.SoftWareCheckPath = comm.G_CliInfo.WorkPath + gCliConf.SoftWareCheckPath
	}
	if !filepath.IsAbs(gCliConf.CfgManageFilePath) {
		gCliConf.CfgManageFilePath = comm.G_CliInfo.WorkPath + gCliConf.CfgManageFilePath
	}
	glog.V(3).Infof("cli-conf=%v", gCliConf)
	return
}

func GetSerConf() *SerConf {
	return gSerConf
}
func GetCliConf() *CliConf {
	return gCliConf
}
