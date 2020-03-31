package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"go.etcd.io/etcd/clientv3"
	"io/ioutil"
	"log"
	"shagt/conf"
	"shagt/pub"
	"strings"
	"time"
)

type EtcdClient struct {
	client *clientv3.Client
}

func NewEtcdClient(addr []string) (*EtcdClient, error) {
	conf := clientv3.Config{
		Endpoints:   addr,
		DialTimeout: 5 * time.Second,
	}
	client, err := clientv3.New(conf)
	if err != nil {
		glog.V(0).Infof("clientv3.New err,%v", err)
		return nil, err
	}
	//comm.G_etcdclient = client
	return &EtcdClient{client: client}, nil
}

func (this *EtcdClient) GetKey(prefix string) (*map[string]string, error) {
	if this == nil {
		return nil, nil
	}
	resp, err := this.client.Get(context.Background(), prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	kv := make(map[string]string)
	for i := range resp.Kvs {
		if v := resp.Kvs[i].Value; v != nil {
			kv[string(resp.Kvs[i].Key)] = string(resp.Kvs[i].Value)
		}
	}
	return &kv, nil
}

//put 不带租约
func (this *EtcdClient) PutKey(key, val string) error {
	kv := clientv3.NewKV(this.client)
	_, err := kv.Put(context.TODO(), key, val)
	return err
}

func (this *EtcdClient) Close() error {
	return this.client.Close()
}

//--------------------------------------------------

//创建租约注册服务
type ServiceReg struct {
	client        *clientv3.Client
	lease         clientv3.Lease
	leaseResp     *clientv3.LeaseGrantResponse
	canclefunc    func()
	keepAliveChan <-chan *clientv3.LeaseKeepAliveResponse
	key           string
}

func (this *EtcdClient) NewServiceReg(timeNum int) (*ServiceReg, error) {
	if this == nil {
		return nil, nil
	}
	ser := &ServiceReg{
		client: this.client,
	}
	if err := ser.setLease(timeNum); err != nil {
		glog.V(0).Infof("clientv3.New err,%v", err)
		return nil, err
	}
	go ser.ListenLeaseRespChan() //for debug
	return ser, nil
}

//设置租约
func (this *ServiceReg) setLease(timeNum int) error {
	lease := clientv3.NewLease(this.client)

	//设置租约时间
	leaseResp, err := lease.Grant(context.TODO(), int64(timeNum))
	if err != nil {
		glog.V(0).Infof("etcd Grant err,%v", err)
		return err
	}

	//设置续租
	ctx, cancelFunc := context.WithCancel(context.TODO())
	leaseRespChan, err := lease.KeepAlive(ctx, leaseResp.ID)
	if err != nil {
		glog.V(0).Infof("etcd KeepAlive err,%v", err)
		return err
	}

	this.lease = lease
	this.leaseResp = leaseResp
	this.canclefunc = cancelFunc
	this.keepAliveChan = leaseRespChan
	return nil
}

//监听 续租情况
func (this *ServiceReg) ListenLeaseRespChan() {
	for {
		select {
		case leaseKeepResp := <-this.keepAliveChan:
			if leaseKeepResp == nil {
				glog.V(3).Info("已经关闭续租功能")
				return
			} else {
				glog.V(3).Info("续租成功")
			}
		}
	}
}

//通过租约 注册服务
func (this *ServiceReg) PutService(key, val string) error {
	kv := clientv3.NewKV(this.client)
	_, err := kv.Put(context.TODO(), key, val, clientv3.WithLease(this.leaseResp.ID))

	return err
}

//撤销租约
func (this *ServiceReg) RevokeLease() error {
	this.canclefunc()
	time.Sleep(2 * time.Second)
	_, err := this.lease.Revoke(context.TODO(), this.leaseResp.ID)
	return err
}

//
//func main() {
//	ser, _ := NewServiceReg([]string{"127.0.0.1:2379"}, 30)
//	ser.PutService("/node/111", "heiheihei")
//	ser.PutService("/node/112", "etcd测试2")
//	select {}
//}
//----------------------------------------
type CliRegInfo struct {
	Hostname string
	Ip       string
	pid      string
	ver      string
	os       string
}

type ClientDis struct {
	Client     *clientv3.Client
	ServerList map[string]CliRegInfo
	NeedFlash  bool
}

func (this *EtcdClient) NewClientDis() *ClientDis {
	if this == nil {
		return nil
	}
	cliDis := ClientDis{
		Client:     this.client,
		ServerList: make(map[string]CliRegInfo),
		NeedFlash:  false,
	}
	file_info := readCliRegInfoFromFile()
	for _, v := range *file_info {
		cliDis.ServerList[v.Hostname] = v
	}
	return &cliDis
}

func readCliRegInfoFromFile() *[]CliRegInfo {
	filepath := conf.GetSerConf().CliRegInfo
	clireginfo := make([]CliRegInfo, 0)
	if ok, err := pub.IsFile(filepath); !ok {
		glog.V(0).Infof("CliRegInfo: %s, err: [%v]", filepath, err)
		return &clireginfo
	}
	buf, err := ioutil.ReadFile(filepath)
	if err != nil {
		glog.V(0).Infof("read file: %s, err: [%v]", filepath, err)
		return &clireginfo
	}
	err = json.Unmarshal(buf, &clireginfo)
	if err != nil {
		glog.V(0).Infof("json.Unmarshal err: [%v]", err)
		return nil
	}

	return &clireginfo
}
func (this *ClientDis) GetService(prefix string) ([]string, error) {
	resp, err := this.Client.Get(context.Background(), prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	addrs := this.extractRegCli(resp)

	go this.watcher(prefix)
	return addrs, nil
}

func (this *ClientDis) watcher(prefix string) {
	rch := this.Client.Watch(context.Background(), prefix, clientv3.WithPrefix())
	for wresp := range rch {
		for _, ev := range wresp.Events {
			switch ev.Type {
			case clientv3.EventTypePut:
				this.SetServiceList(string(ev.Kv.Key), string(ev.Kv.Value))
			case clientv3.EventTypeDelete:
				this.DelServiceList(string(ev.Kv.Key))
			}
		}
	}
}

func (this *ClientDis) extractRegCli(resp *clientv3.GetResponse) []string {
	addrs := make([]string, 0)
	if resp == nil || resp.Kvs == nil {
		return addrs
	}
	for i := range resp.Kvs {
		if v := resp.Kvs[i].Value; v != nil {
			this.SetServiceList(string(resp.Kvs[i].Key), string(resp.Kvs[i].Value))
			addrs = append(addrs, string(v))
		}
	}
	return addrs
}

//val format:hostname,ip,pid,ver,os
func (this *ClientDis) SetServiceList(key, val string) {
	i := strings.LastIndex(key, "/")
	if i >= 0 {
		key = key[i+1:]
	}
	valArray := strings.Split(val, ",")
	newCliinfo := CliRegInfo{
		Hostname: valArray[0],
		Ip:       valArray[1],
		pid:      valArray[2],
		ver:      valArray[3],
		os:       valArray[4],
	}
	glog.V(3).Infof("discover new service, key :%s,val:%s", key, val)
	//this.ServerList[key] = string(val)
	this.ServerList[key] = newCliinfo
	this.NeedFlash = true
}

func (this *ClientDis) DelServiceList(key string) {
	i := strings.LastIndex(key, "/")
	if i >= 0 {
		key = key[i+1:]
	}
	delete(this.ServerList, key)
	this.NeedFlash = true
	log.Println("del data key:", key)
}

func (this *ClientDis) SerList2Array(host string) []string {
	addrs := make([]string, 0)

	for _, v := range this.ServerList {
		if len(host) == 0 || strings.Contains(v.Hostname, host) {
			s := fmt.Sprintf("%s,%s,%s,%s", v.Hostname, v.Ip, v.pid, v.ver, v.os)
			addrs = append(addrs, s)
		}
	}
	return addrs
}

//func main() {
//	cli, _ := NewClientDis([]string{"127.0.0.1:2379"})
//	cli.GetService("/node")
//
//	select {}
//}
