package etcd

import (
	"github.com/coreos/etcd/clientv3"
	"time"
	"fmt"
	"ufleet/launcher/code/config"
	"golang.org/x/net/context"
	"strings"
)

type LEtcd struct {
	client *clientv3.Client
	ctx    context.Context
}

func (le *LEtcd) Init(IPs []string) error {
	endpoints := make([]string, 0, 4)
	for i := range IPs{
		endpoints = append(endpoints, fmt.Sprintf("http://%s:%d", IPs[i], config.GK8sDefault.EtcdListenPort))
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second, // time out
	})
	if err != nil {
		// handle error!
		return err
	}
	if cli == nil {
		return fmt.Errorf("can't init etcd client v3")
	}
	le.client = cli
	le.ctx    = context.Background()
	return nil
}

func (le *LEtcd) InitByEndpoints(endpoints []string) error {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second, // time out
	})
	if err != nil {
		// handle error!
		return err
	}
	if cli == nil {
		return fmt.Errorf("can't init etcd client v3")
	}
	le.client = cli
	le.ctx    = context.Background()
	return nil
}

func (le *LEtcd) Release(){
	if le != nil && le.client != nil {
		le.client.Close()
	}
}

func (le *LEtcd) MemberAdd(IP string) error {
	if le == nil || le.client == nil {
		return fmt.Errorf("etcd: should init first")
	}

	membersResp, err := le.client.MemberList(le.ctx)
	if err != nil {
		return err
	}
	if membersResp == nil {
		return fmt.Errorf("no member found")
	}
	for i := range membersResp.Members {
		for j:= range membersResp.Members[i].ClientURLs {
			if strings.Contains(membersResp.Members[i].ClientURLs[j], fmt.Sprintf("//%s:", IP)) {
				// already add
				return nil
			}
		}
		for j:= range membersResp.Members[i].PeerURLs {
			if strings.Contains(membersResp.Members[i].PeerURLs[j], fmt.Sprintf("//%s:", IP)) {
				// already add but etcd node not join
				return nil
			}
		}
	}

	_, err = le.client.MemberAdd(le.ctx, []string{fmt.Sprintf("http://%s:%d", IP, config.GK8sDefault.EtcdPeerPort)})
	return err
}

func (le *LEtcd) MemberRemove(IP string) error {
	if le == nil || le.client == nil {
		return fmt.Errorf("etcd: should init first")
	}

	cli := le.client

	membersResp, err := cli.MemberList(le.ctx)
	if err != nil {
		// failed to get member
		return err
	}
	if membersResp == nil {
		// no member found
		return nil
	}

	isExist := false
	var id uint64
	for i := range membersResp.Members {
		if strings.Contains(membersResp.Members[i].Name, IP) {
			id = membersResp.Members[i].ID
			isExist = true
		}
	}
	if !isExist {
		// not exist
		return nil
	}

	_, err = cli.MemberRemove(le.ctx, id)

	return err
}

func (le *LEtcd) WaitUntilReady() error {
	if le == nil || le.client == nil {
		return fmt.Errorf("etcd: should init first")
	}

	retryTimes := config.GK8sDefault.TimesOfCheckApiserver

	le.client.Put(le.ctx, "/ping", "true")
	ok := false
	for i:=0;i<retryTimes;i++ {
		le.client.Delete(le.ctx, "/ping", clientv3.WithPrefix())
		_, err := le.client.Put(le.ctx, "/ping", "true")
		if err == nil {
			ok = true
			break
		}
		time.Sleep(time.Second)
	}
	le.client.Delete(le.ctx, "/ping")
	if !ok {
		return fmt.Errorf("etcd cluster not ready or check etcd status time out")
	}

	return nil
}

func (le *LEtcd) GetIpByName(name string) string {
	if le == nil || le.client == nil {
		return ""
	}

	membersResp, err := le.client.MemberList(le.ctx)
	if err != nil {
		return ""
	}
	for _, member := range membersResp.Members {
		if member.Name != name {
			continue
		}
		for i := range member.ClientURLs {
			points := member.ClientURLs[i]
			points  = points[7:] // 去掉 http://
			ipAndport := strings.Split(points, ":")
			if len(ipAndport) >= 2 {
				// TODO 可能需要检查是否符合IP格式，防止意料外的错误
				return ipAndport[0]
			}
		}
	}

	return ""
}

func (le *LEtcd) Get(key string) (map[string]string, error) {
	if le == nil || le.client == nil {
		return nil, fmt.Errorf("etcd: should init first")
	}

	result := make(map[string]string)

	gr, err := le.client.Get(le.ctx, key)
	if err == nil {
		if gr != nil {
			for i := range gr.Kvs {
				result[string(gr.Kvs[i].Key)] = string(gr.Kvs[i].Value)
			}
		}
	}

	// get the key and follow
	suffix := ""
	newKey := ""
	if len(key) >= 2 { // maybe this is no need, because launcher's key length more than 2 characters
		suffix = key[len(key) - 2:len(key) - 1]
	} else {
		return result, err
	}
	if suffix != "/" {
		newKey = key + "/"
	}else{
		newKey = key
	}

	gr, err = le.client.Get(le.ctx, newKey, clientv3.WithPrefix())
	if err == nil {
		if gr != nil {
			for i := range gr.Kvs {
				result[string(gr.Kvs[i].Key)] = string(gr.Kvs[i].Value)
			}
		}
	}

	return result, err
}

func (le *LEtcd) Put(key, value string) error {
	if le == nil || le.client == nil {
		return fmt.Errorf("etcd: should init first")
	}

	// if already exist, just return
	// read more, write less for speed up
	gr, err := le.client.Get(le.ctx, key)
	if err == nil {
		if gr != nil {
			for i := range gr.Kvs {
				if i%2 == 0 {
					continue
				}
				if strings.Compare(string(gr.Kvs[i].Value), value) == 0 {
					// already set, just return
					return nil
				}
			}
		}
	}

	_, err = le.client.Put(le.ctx, key, value)
	return err
}

func (le *LEtcd) Delete(key string) error {
	if le == nil || le.client == nil {
		return fmt.Errorf("etcd: should init first")
	}

	// delete the key
	_, err1 := le.client.Delete(le.ctx, key)

	// delete the key and follow
	suffix := ""
	newKey := ""
	if len(key) >= 2 { // maybe this is no need, because launcher's key length more than 2 characters
		suffix = key[len(key) - 2:len(key) - 1]
	} else {
		return err1
	}
	if suffix != "/" {
		newKey = key + "/"
	}else{
		newKey = key
	}

	_, err2 := le.client.Delete(le.ctx, newKey, clientv3.WithPrefix())
	if err1 != nil || err2 != nil {
		err := fmt.Errorf("%v %v", err1, err2)
		return err
	}
	return nil
}

func (le *LEtcd) IsKey(key string) (bool, error) {
	if le == nil {
		return false, fmt.Errorf("etcd: should init first")
	}

	values, err := le.Get(key)
	if err != nil {
		return false, err
	}

	if values != nil && (len(values) >= 1) {
		return true, nil
	}

	return false, nil
}
