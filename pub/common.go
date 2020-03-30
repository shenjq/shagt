package pub

import (
	"bytes"
	"fmt"
	"github.com/golang/glog"
	"os/exec"
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
	glog.V(0).Infof("%v start", cd.Args)
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
	glog.V(0).Infof("out:%v", o)
	glog.V(0).Infof("err:%v", e)

	glog.V(0).Infof("%v ....done", cd.Args)
	return strings.ReplaceAll(o.String(), "\n", ""), nil
}

const (
	timeTemplate1 = "2006-01-02 15:04:05" //常规类型
	timeTemplate2 = "2006/01/02 15:04:05" //其他类型
	timeTemplate3 = "2006-01-02"          //其他类型
	timeTemplate4 = "20060102"
	timeTemplate5 = "15:04:05" //其他类型
)

//timeTemplate1 = "2006-01-02 15:04:05"
func Timestamp2Str1(d int64) string {
	return time.Unix(d, 0).Format(timeTemplate1)
}
