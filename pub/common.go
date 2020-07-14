package pub

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func MathTrunc(f float64, n int) float64 {
	fmtstr := fmt.Sprintf("%%.%df", n)
	value_s := fmt.Sprintf(fmtstr, f)
	value_f, err := strconv.ParseFloat(value_s, 64)
	if err != nil {
		glog.V(0).Infof("ParseFloat err,%v", err)
		return 0
	}
	return value_f
}

//返回os命令返回结果
func ExecOSCmd(cmd string) (string, error) {
	cmdarray := strings.Split(cmd, " ")
	if len(cmdarray) == 0 {
		return "", fmt.Errorf("命令行为空")
	}
	args := make([]string, 0)
	for i, v := range cmdarray {
		if i == 0 {
			continue
		}
		args = append(args, v)
	}

	cd := exec.Command(cmdarray[0], args...)
	glog.V(2).Infof("ExecOSCmd: %v", cd.Args)
	var o, e bytes.Buffer
	cd.Stdout = &o
	cd.Stderr = &e
	err := cd.Start()
	if err != nil {
		glog.V(0).Infof("exec.start err, %v", err)
		return "", err
	}
	err = cd.Wait()
	if err != nil {
		glog.V(0).Infof("exec.Command err, %v", err)
		return "", err
	}
	glog.V(4).Infof("out:%v", o)
	glog.V(4).Infof("err:%v", e)

	glog.V(2).Infof("%v ....done", cd.Args)
	return o.String(), nil
}

const (
	timeTemplate1 = "2006-01-02 15:04:05" //常规类型
	timeTemplate2 = "2006/01/02 15:04:05" //其他类型
	timeTemplate3 = "2006-01-02"          //其他类型
	timeTemplate4 = "20060102"
	timeTemplate5 = "15:04:05" //其他类型
	timeTemplate6 = "20060102150405"
)

//timeTemplate1 = "2006-01-02 15:04:05"
func Timestamp2Str1(d int64) string {
	return time.Unix(d, 0).Format(timeTemplate1)
}

func GetTimeStr1() string {
	return time.Now().Format(timeTemplate1)
}
func GetTimeStr6() string {
	return time.Now().Format(timeTemplate6)
}

func GetCurrentPath() (string, error) {
	file, err := exec.LookPath(os.Args[0])
	if err != nil {
		return "", err
	}
	path, err := filepath.Abs(file)
	if err != nil {
		return "", err
	}

	if runtime.GOOS == "windows" {
		path = strings.Replace(path, "\\", "/", -1)
	}

	i := strings.LastIndex(path, "/")
	if i < 0 {
		return "", errors.New(`Can't find "/" or "\".`)
	}

	return string(path[0 : i+1]), nil
}

func GetWorkPath() (string, error) {
	file, err := exec.LookPath(os.Args[0])
	if err != nil {
		return "", err
	}
	path, err := filepath.Abs(file)
	if err != nil {
		return "", err
	}

	if runtime.GOOS == "windows" {
		path = strings.Replace(path, "\\", "/", -1)
	}

	i := strings.LastIndex(path, "/")
	if i < 0 {
		return "", errors.New("目录格式有误")
	} else if i == 0 {
		return "/", nil
	}
	j := strings.LastIndex(path[0:i], "/")
	if j < 0 {
		return "", errors.New("目录格式有误")
	}
	return string(path[0 : j+1]), nil
}

func GetHostName() string {
	h1, err := os.Hostname()
	if err != nil {
		glog.V(0).Infof("get hostname err,%v", err)
		return ""
	}
	if strings.HasPrefix(strings.ToUpper(h1), "C") { //匹配CAxxxx、CBxxxx
		return h1
	}
	var h2 string
	glog.V(0).Infof("get hostname os:[%s]", runtime.GOOS)
	if runtime.GOOS == "linux" {
		b, err := ioutil.ReadFile("/etc/issue")
		if err != nil {
			glog.V(0).Infof("ioutil.ReadFile err,%v", err)
		} else if strings.HasPrefix(string(b), "C") { //匹配CAxxxx、CBxxxx
			h2 = string(b)
		}
	}
	if len(h2) > 0 {
		return h2
	} else { //这种情况下还是返回h1
		return h1
	}
}

func GetMgrIP(target string) string {
	conn, err := net.Dial("udp", target)
	if err != nil {
		glog.V(0).Infof("net.Dial err,%v", err)
		return ""
	}
	defer conn.Close()
	return strings.Split(conn.LocalAddr().String(), ":")[0]
}
