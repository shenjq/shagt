package pub

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
)

type FileMd5Stru struct {
	Filepath string `json:"filepath"`
	Md5str   string `json:"md5str"`
	Note     string `json:"note"`
}

var gFileMd5list_last *[]FileMd5Stru

//根据配置文件检查文件是否有变动
//返回有变动文件列表、md5str、说明
func CheckFile(cfgfile string) (*[]FileMd5Stru, error) {
	//读取配置文件,获取全文件列表
	allFileList := getAllFileList(cfgfile)
	glog.V(3).Infof("allFilelist:%v", allFileList)

	//计算当前文件列表md5
	FileMd5List_now := GetFileListMd5(allFileList)
	glog.V(3).Infof("FileMd5List_now:%v", FileMd5List_now)

	//对比之前，输出结果add、del、up
	if gFileMd5list_last == nil {
		gFileMd5list_last = readMd5ResultFile(cfgfile + ".result")
	}
	glog.V(3).Infof("gFileMd5list_last:%v", gFileMd5list_last)

	result := compareFileMd5List(FileMd5List_now, gFileMd5list_last)
	glog.V(3).Infof("result:%v", result)
	gFileMd5list_last = FileMd5List_now
	if len(*result) > 0 {
		flashMd5ResultFile(cfgfile + ".result")
	}
	return result, nil
}
func flashMd5ResultFile(resultfile string) {
	buf, err := json.Marshal(gFileMd5list_last)
	if err != nil {
		glog.V(0).Infof("json.Marshal err: [%v]", err)
		return
	}
	syscall.Umask(0000)
	err = ioutil.WriteFile(resultfile, buf, 0600)
	if err != nil {
		glog.V(0).Infof("ioutil.WriteFile failure, err=[%v]\n", err)
	}
}
func readMd5ResultFile(resultfile string) *[]FileMd5Stru {
	last := make([]FileMd5Stru, 0)
	if ok, _ := IsFile(resultfile); !ok {
		return &last
	}
	buf, err := ioutil.ReadFile(resultfile)
	if err != nil {
		glog.V(0).Infof("read file: %s, err: [%v]", resultfile, err)
		return &last
	}
	err = json.Unmarshal(buf, &last)
	if err != nil {
		glog.V(0).Infof("json.Unmarshal err: [%v]", err)
		return nil
	}
	return &last
}

func compareFileMd5List(now, last *[]FileMd5Stru) *[]FileMd5Stru {
	now_map := make(map[string]string)
	last_map := make(map[string]string)
	union := make(map[string]string)
	result := make([]FileMd5Stru, 0)

	for _, v := range *last {
		last_map[v.Filepath] = v.Md5str
		union[v.Filepath] = v.Md5str
	}
	for _, v := range *now {
		now_map[v.Filepath] = v.Md5str
		union[v.Filepath] = v.Md5str
	}
	for k, v := range union {
		v_now, ok_now := now_map[k]
		v_last, ok_last := last_map[k]
		var note string
		if v == v_last && ok_now && ok_last {
			continue
		}
		if v != v_last && ok_now && ok_last { //up
			note = "update"
		}
		if v == v_last && !ok_now && ok_last { //del
			note = "del"
		}
		if v == v_now && ok_now && !ok_last { //add
			note = "add"
		}
		entry := FileMd5Stru{
			Filepath: k,
			Md5str:   v,
			Note:     note,
		}
		result = append(result, entry)
	}
	return &result
}

func getAllFileList(cfgfile string) *[]string {
	allFileList := make([]string, 0)
	file, err := os.Open(cfgfile)
	if err != nil {
		glog.V(0).Infof("Open file: %s, err: [%v]", cfgfile, err)
		return &allFileList
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		for _, v := range *ListFile(line) {
			allFileList = append(allFileList, v)
		}
	}
	return &allFileList
}

func GetFileMd5Str(file string) (md5str string, err error) {
	var f *os.File
	f, err = os.Open(file)
	if err != nil {
		glog.V(0).Infof("Open file: %s, err: [%v]", file, err)
		return
	}
	defer f.Close()
	md5 := md5.New()
	if _, err = io.Copy(md5, f); err != nil {
		glog.V(0).Infof("io.Copy err: [%v]", err)
		return
	}
	return hex.EncodeToString(md5.Sum(nil)), nil
}

func GetFileListMd5(flist *[]string) *[]FileMd5Stru {
	filemd5arry := make([]FileMd5Stru, 0)
	for _, v := range *flist {
		md5str, err := GetFileMd5Str(v)
		if err != nil {
			glog.V(0).Infof("GetFileMd5Str err: %v", err)
			continue
		}
		fmd5 := FileMd5Stru{
			Filepath: v,
			Md5str:   md5str,
		}
		filemd5arry = append(filemd5arry, fmd5)
	}
	return &filemd5arry
}

func ListFile(fpath string) *[]string {
	flist := make([]string, 0)
	isfile, err := IsFile(fpath)
	if err != nil { //不存在，或匹配符,此处只考虑两种情况:xxx*、*xxx
		glog.V(3).Infof("fpath:%s not exist,%v\n", fpath, err)
		if strings.Count(fpath, "*") > 0 {
			lpf := listPatternFile(fpath)
			if len(*lpf) > 0 {
				for _, v := range *lpf {
					flist = append(flist, v)
				}
			}
			glog.V(3).Infof("lpf:%v\n", lpf)
		}
		return &flist
	}
	//file
	if isfile {
		glog.V(3).Infof("fpath:%s is a file\n", fpath)
		flist = append(flist, fpath)
		return &flist
	}
	//目录,遍历该目录所有文件
	glog.V(3).Infof("fpath:%s is a dir\n", fpath)
	ldf := listDirFile(fpath, -1)
	if len(*ldf) > 0 {
		for _, v := range *ldf {
			flist = append(flist, v)
		}
	}
	glog.V(3).Infof("lpf:%v\n", ldf)
	return &flist
}

func IsFile(f string) (bool, error) {
	fi, err := os.Stat(f)
	if err != nil {
		return false, err
	}
	return !fi.IsDir(), nil
}

//level表示要比例该目录下目录的层数,0-表示只遍历该目录文件，不下钻,-1表示遍历所有
func listDirFile(dir string, level int) *[]string {
	flist := make([]string, 0)
	fileinfo, err := ioutil.ReadDir(dir)
	if err != nil {
		glog.V(0).Infof("ioutil.ReadDir err,%v", err)
		return &flist
	}
	for _, file := range fileinfo {
		//f := fmt.Sprintf("%s%s", dir, file.Name())
		f := filepath.Join(dir, file.Name())
		if file.IsDir() {
			if level != 0 {
				for _, ldf := range *listDirFile(f, level-1) {
					flist = append(flist, ldf)
				}
			} else {
				continue
			}
		} else {
			flist = append(flist, f)
		}
	}
	return &flist
}

//列出*匹配的文件列表
func listPatternFile(dir string) *[]string {
	flist := make([]string, 0)
	d, f := path.Split(dir)
	dirfile := listDirFile(d, 0)
	if len(*dirfile) == 0 {
		return &flist
	}
	glog.V(3).Infof("=====>%v", dirfile)
	if strings.HasPrefix(f, "*") { //*xxx格式
		glog.V(3).Infof("start with *")
		for _, v := range *dirfile {
			_, f2 := path.Split(v)
			if strings.HasSuffix(f2, f[1:]) {
				flist = append(flist, v)
			}
		}
	} else if strings.HasSuffix(f, "*") { //xxx*格式
		glog.V(3).Infof("end with *")
		for _, v := range *dirfile {
			_, f2 := path.Split(v)
			if strings.HasPrefix(f2, f[0:len(f)-1]) {
				flist = append(flist, v)
			}
		}
	} else {
		fmt.Printf("err\n")
		glog.V(0).Infof("file:%s pattern err", dir)
	}
	return &flist
}
