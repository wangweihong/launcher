package base

/**
 * 集中处理常见异常
 * 1. 服务端reset连接
 */

import (
	"ufleet/launcher/code/model/base/interf"
	"ufleet/launcher/code/model/base/impl/etcd"
	"ufleet/launcher/code/model/base/impl/bolt"
	"fmt"
	"strings"
	"log"
	"ufleet/launcher/code/config"
)

var iDB interf.IDB
var iDBArgs []string

func InitDB(args ...string) error {
	var err error
	var tEtcd etcd.Etcd
	var tBolt bolt.Bolt

	if len(args) > 0 {
		iDBArgs = args
	}

	// select and init db
	switch config.GDefault.ServiceType {
	case "server":
		var endpoints []string
		if len(args) > 0 {
			endpoints = args
		} else {
			if iDBArgs != nil {
				endpoints = iDBArgs
			} else {
				endpoints = []string{fmt.Sprintf("http://%s:%d", config.GDefault.HostIP, config.GDefault.PortEtcd)}
				iDBArgs = endpoints
			}
		}
		iDB, err = tEtcd.New(endpoints...)
		if err != nil {
			log.Printf("can't init etcd by etcd endpoint(%v): %s", endpoints, err.Error())
			return fmt.Errorf("can't init etcd by etcd endpoint(%v): %s", endpoints, err.Error())
		}
	case "module":
		iDB, err = tBolt.New(args...)
		if err != nil {
			errMsg := fmt.Sprintf("init bolt server failed with args: %v", args)
			log.Printf(errMsg)
			return fmt.Errorf(errMsg)
		}
		iDBArgs = args
	}

	return nil
}

// TODO 考虑在Ha模式下，可能会出现Ufleet主机异常，切换主机的情况，故可能需要重连Etcd节点
// 重新封装　Get当出现某些错误时，重新连接
func Get(key string) (map[string]string, error){
	if iDB == nil {
		if err := InitDB(); err != nil {
			return nil, err
		}
	}

	result, err := iDB.Get(key)
	if err != nil && strings.Contains(err.Error(), "reset") {
		// 服务端reset，意味之前的连接已无效，需要重新创建连接
		ierr := InitDB()
		if ierr != nil {
			return nil, fmt.Errorf("because of %s, connect etcd server again but failed: %s", err.Error(), ierr.Error())
		}

		result, err = iDB.Get(key)
	}

	return result, nil
}

func IsExist(key string) (bool, error) {
	if iDB == nil {
		if err := InitDB(); err != nil {
			return false, err
		}
	}

	values, err := Get(key)
	if err != nil {
		return false, err
	}

	if values == nil || len(values) == 0 {
		return false, nil
	}

	return true, nil
}

// 重新封装 Put 当出现某些错误时，重新连接
func Put(key,value string) error {
	if iDB == nil {
		if err := InitDB(); err != nil {
			return err
		}
	}

	err := iDB.Put(key, value)
	if err != nil && strings.Contains(err.Error(), "reset") {
		// 服务端reset，意味之前的连接已无效，需要重新创建连接
		ierr := InitDB()
		if ierr != nil {
			return fmt.Errorf("because of %s, connect etcd server again but failed: %s", err.Error(), ierr.Error())
		}

		err = iDB.Put(key, value)
	}

	return err
}

func Delete(key string) error {
	if iDB == nil {
		if err := InitDB(); err != nil {
			return err
		}
	}

	err := iDB.Delete(key)
	if err != nil && strings.Contains(err.Error(), "reset") {
		// 服务端reset，意味之前的连接已无效，需要重新创建连接
		eerr := InitDB()
		if eerr != nil {
			return fmt.Errorf("because of %s, connect etcd server again but failed: %s", err.Error(), eerr.Error())
		}

		err = iDB.Delete(key)
	}

	return err
}
