package clilogmon

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/hpcloud/tail"
	"io/ioutil"
	"shagt/comm"
	"shagt/conf"
	"shagt/pub"
	"strings"
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
			continue
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
	var flag int8
	var keywords []string
	if len(le.CliLogConf.Keyword) == 0 {
		flag = 1
	} else if strings.Contains(le.CliLogConf.Keyword, "WARNTOEC") {
		flag = 2
	} else {
		flag = 3
		keywords = strings.Split(le.CliLogConf.Keyword, "|")
	}

	for ; ; {
		select {
		case line := <-le.LogTail.Lines:
			glog.V(3).Infof("read data:%s", line.Text)
			catch := false
			//发送预警信息
			if flag == 2 {
				if strings.Contains(line.Text, "WARNTOEC") {
					catch = true
				}
			} else if flag == 3 {
				for _, k := range keywords {
					if strings.Contains(line.Text, k) {
						catch = true
						break
					}
				}
			} else {
				catch = true
			}
			if catch {
				handleLog(flag, &le.CliLogConf, line.Text)
			}
		default:
			time.Sleep(time.Second)
		}
	}
}

func handleLog(flag int8, logconf *CliLogConf, text string) {
	type warninfo struct {
		Id_original string `json:"id_original"`
		Ip          string `json:"ip"`
		Severity    string `json:"severity"`
		Title       string `json:"title"`
		Summary     string `json:"summary"`
		Status      string `json:"status"`
	}
	winfo := new(warninfo)
	if flag == 1 || flag == 3 {
		winfo.Summary = fmt.Sprintf("日志文件%s发现预警信息:%s", logconf.Path, text)
	} else {
		i1 := strings.Index(text, "{")
		i2 := strings.LastIndex(text, "}")
		if i1 < 0 || i2 < 0 {
			glog.V(0).Infof("预警格式有误,i1=%d,i2=%d\n", i1,i2)
			winfo.Summary = fmt.Sprintf("日志文件%s发现WARNTOEC预警,但预警格式有误!", logconf.Path)
		} else {
			logstr := text[i1:i2+1]
			err := json.Unmarshal([]byte(logstr), &winfo)
			if err != nil {
				glog.V(0).Infof("预警格式[%s]有误,%v\n", logstr,err)
				winfo.Summary = fmt.Sprintf("日志文件%s,发现WARNTOEC预警,但预警格式有误!", logconf.Path)
			}
		}
	}
	winfo.Title = fmt.Sprintf("日志文件%s发现预警信息", logconf.Path)
	if len(logconf.System) > 0 && len(winfo.Id_original) == 0 {
		winfo.Id_original = logconf.System
	}
	winfo.Ip = conf.GetCliConf().LocalHostIp

	upcmUrl := fmt.Sprintf("http://%s:17788/warn", comm.G_ReadFromServerConf.ServerAddress)
	jsonbytes, _ := json.Marshal(winfo)
	glog.V(3).Infof("提交预警事件信息:%s", string(jsonbytes))
	r, err := pub.PostJson(upcmUrl, string(jsonbytes))
	glog.V(3).Infof("result:%s", r)
	if err != nil {
		glog.V(0).Infof("提交失败:%v", err)
	}
}
