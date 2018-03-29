package etcd

import (
	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
	"time"
	"fmt"
	"ufleet/launcher/code/config"
	"strings"
)

type LEtcdv2 struct {
	MememberCli     client.MembersAPI
	KeysCli         client.KeysAPI
	Context         context.Context
}

func (le *LEtcdv2) Init(IPs []string, port int) error {
	endpoints := make([]string, 0, 4)
	for i := range IPs{
		endpoints = append(endpoints, fmt.Sprintf("http://%s:%d", IPs[i], port))
	}

	// 获取etcd连接
	cfg := client.Config{
		Endpoints:               endpoints,
		Transport:               client.DefaultTransport,
		// set timeout per request to fail fast when the target endpoint is unavailable
		HeaderTimeoutPerRequest: time.Second * 10,
	}

	etcdClient, err := client.New(cfg)
	if err != nil {
		return err
	}

	memCli := client.NewMembersAPI(etcdClient)
	keysCli := client.NewKeysAPI(etcdClient)
	ctx  := context.Background()

	if memCli == nil || keysCli == nil || ctx == nil {
		return fmt.Errorf("can't get etcd member api or background context")
	}

	le.MememberCli = memCli
	le.KeysCli = keysCli
	le.Context = ctx
	return nil
}

func (le *LEtcdv2) Add(IP string) error {
	cli := le.MememberCli

	members, err := cli.List(le.Context)
	if err != nil {
		// failed to get member
		return err
	}

	for i := range members {
		if strings.Contains(members[i].Name, IP) {
			// already add
			return nil
		}
	}

	_, err = cli.Add(le.Context, fmt.Sprintf("http://%s:%d", IP, config.GK8sDefault.EtcdPeerPort))
	return err
}

func (le *LEtcdv2) Remove(IP string) error {
	cli := le.MememberCli

	members, err := cli.List(le.Context)
	if err != nil {
		// failed to get member
		return err
	}

	isExist := false
	id := ""
	for i := range members {
		if strings.Contains(members[i].Name, IP) {
			id = members[i].ID
			isExist = true
		}
	}
	if !isExist {
		// not exist
		return nil
	}

	err = cli.Remove(le.Context, id)

	return err
}

func (le *LEtcdv2) WaitForReady() error {
	retryTimes := config.GK8sDefault.TimesOfCheckEtcd

	opts := client.SetOptions{Dir: false}
	deleteOpts := client.DeleteOptions{Dir: false}
	ok := false
	for i:=0;i<retryTimes;i++ {
		le.KeysCli.Delete(le.Context, "/ping", &deleteOpts)
		_, err := le.KeysCli.Set(le.Context, "/ping", "true", &opts)
		if err == nil {
			ok = true
			break
		}
		time.Sleep(time.Second)
	}
	le.KeysCli.Delete(le.Context, "/ping", &deleteOpts)
	if !ok {
		return fmt.Errorf("etcd cluster not ready or check etcd status time out")
	}

	return nil
}

func (le *LEtcdv2) GetIpByName(name string) (string, error) {
	members, err := le.MememberCli.List(le.Context)
	if err != nil {
		return "", err
	}
	for _, member := range members {
		if member.Name != name {
			continue
		}
		for i := range member.ClientURLs {
			points := member.ClientURLs[i]
			points  = points[7:] // 去掉 http://
			ipAndport := strings.Split(points, ":")
			if len(ipAndport) >= 2 {
				// TODO 可能需要检查是否符合IP格式，防止意料外的错误
				return ipAndport[0], nil
			}
		}
	}

	return "", nil
}
