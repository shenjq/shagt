package comm

import (
	"errors"
	"github.com/golang/glog"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

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
