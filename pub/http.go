package pub

import (
	"bytes"
	"encoding/json"
	"github.com/golang/glog"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

type Resp struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
}

func (r *Resp) Resp(w http.ResponseWriter) {
	if err := json.NewEncoder(w).Encode(r); err != nil {
		glog.V(0).Infof("%v\n", err)
	}
}

func Middle(f http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer glog.V(3).Infof("Middle request: %s cost %.4f seconds\n", r.URL.String(), time.Now().Sub(start).Seconds())
		f(w, r)
	})
}

// 发送GET请求
// url：         请求地址
// response：    请求返回的内容
func Get(url string) (string, error) {

	// 超时时间：2秒
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		glog.V(0).Infof("http get err,%v\n", err)
		return "", err
	}
	defer resp.Body.Close()

	var buffer = make([]byte, 128, 1024)
	result := bytes.NewBuffer(nil)
	for {
		n, err := resp.Body.Read(buffer[0:])
		result.Write(buffer[0:n])
		if err != nil && err == io.EOF {
			break
		} else if err != nil {
			glog.V(0).Infof("http get err,%v\n", err)
			return "", err
		}
	}
	return result.String(), nil
}

//dataformat id=1000001&value=20.4&host=ipaddress
func PostForm(url string, data string) (string,error) {
	contentType := "application/x-www-form-urlencoded"
	return post(url, contentType, strings.NewReader(data))
}

//dataformat json.Unmarshal([]byte(jsonStr), &data) ##data struct
func PostJson(url string, data interface{}) (string,error) {
	contentType := "application/json"
	jsonStr, _ := json.Marshal(data)
	return post(url, contentType, bytes.NewBuffer(jsonStr))
}

// 发送POST请求
// url：         请求地址
// body：        POST请求提交的数据
// contentType： 请求体格式，如：application/json,application/x-www-form-urlencoded
// return：     请求放回的内容
func post(url string, contentType string, body io.Reader) (string,error) {

	//fmt.Printf("[%v]\n", body)
	// 超时时间：5秒
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(url, contentType, body)
	if err != nil {
		glog.V(0).Infof("http post err,%v\n", err)
		return "",err
	}
	defer resp.Body.Close()

	result, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "",err
	}
	return string(result),nil
}
