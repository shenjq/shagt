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
		return ""
	}
	h1 = strings.ToUpper(h1)
	if strings.HasPrefix(h1, "C") {
		return h1
	}
	if runtime.GOOS == "linux" {
		b, err := ioutil.ReadFile("/etc/issue")
		if err != nil {
			return ""
		}
		if strings.HasPrefix(string(b), "C") {
			return string(b)
		}
	}
	return ""
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
