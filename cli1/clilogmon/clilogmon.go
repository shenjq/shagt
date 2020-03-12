package clilogmon

import (
	"shagt/conf"
	"encoding/json"
	"github.com/golang/glog"
	"github.com/hpcloud/tail"
	"io/ioutil"
	"time"
)

type CliLogConf struct {
	Path    string `json:"path"`
	Keyword string `json:"keyword"`
	Note    string `json:"note"`
	System  string `json:"system"`
}

type CliLogEntry struct {
	CliLogConf
	LogTail *tail.Tail
}

var gCliLogConf = make([]CliLogConf, 0, 3)
var gCliLogEntry = make([]CliLogEntry, 0, 3)

func GetCliLogEntry() []CliLogEntry {
	return gCliLogEntry
}
func GetCliLogConf() []CliLogConf {
	return gCliLogConf
}

//日志初始化
func CliLogMonInit() (err error) {
	if err = getLocalLogConf(); err != nil {
		glog.V(0).Infof("getLocalLogConf err,%v", err)
		return
	}
	for i, li := range gCliLogConf {
		glog.V(3).Infof("i=%d,%v", i, li)
		le, err := li.newClilogEntry()
		if err != nil {
			glog.V(0).Infof("newClilogEntry err,%v", err)
			return err
		}
		go le.watchlog()
		gCliLogEntry = append(gCliLogEntry, *le)
	}
	return nil
}

//读取本地日志配置文件
func getLocalLogConf() error {
	logmonstr, err := ioutil.ReadFile(conf.GetCliConf().CliLogMonPath)
	if err != nil {
		glog.V(0).Infof("ioutil.ReadFile err,%v", err)
		return err
	}
	if err = json.Unmarshal(logmonstr, &gCliLogConf); err != nil {
		glog.V(0).Infof("json.Unmarshal err,%v", err)
		return err
	}
	glog.V(3).Infof("%v", gCliLogConf)
	return nil
}

func (this *CliLogConf) newClilogEntry() (le *CliLogEntry, err error) {
	le = &CliLogEntry{
		CliLogConf: *this,
		LogTail:    nil,
	}
	cfg := tail.Config{
		Location:    &tail.SeekInfo{Offset: 0, Whence: 2}, //0文件的开始,1当前位置,2文件结尾
		ReOpen:      true,                                 //true则文件被删掉阻塞等待新建该文件，false则文件被删掉时程序结束
		MustExist:   false,
		Poll:        true,
		Pipe:        false,
		RateLimiter: nil,
		Follow:      true, //true则一直阻塞并监听指定文件，false则一次读完就结束程序
		MaxLineSize: 0,
		Logger:      nil,
	}
	le.LogTail, err = tail.TailFile(this.Path, cfg)
	if err != nil {
		glog.V(0).Infof("tailfile err,%v", err)
		return
	}
	return
}

func (le *CliLogEntry) watchlog() {
	glog.V(3).Infof("sendlog:%v", le)
	for ; ; {
		select {
		case line := <-le.LogTail.Lines:
			glog.V(3).Infof("read data:%s", line.Text)
		default:
			time.Sleep(time.Millisecond * 50)
		}
	}
}
