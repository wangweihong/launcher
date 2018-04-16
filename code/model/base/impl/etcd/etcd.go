package etcd

import (
	. "ufleet/launcher/code/model/base/interf"

	"ufleet/launcher/code/model/3party/etcd"
)

type Etcd struct {
	client etcd.LEtcd
}

func (b *Etcd) New(endpoints ...string) (IDB, error) {
	le := Etcd{client: etcd.LEtcd{}}
	err := le.client.InitByEndpoints(endpoints)
	if err != nil {
		return nil, err
	}
	return &le, nil
}

func (b *Etcd) Close() {
	b.client.Release()
}

func (b *Etcd) Get(key string) (map[string]string, error) {
	return b.client.Get(key)
}

func (b *Etcd) Put(key, value string) error {
	return b.client.Put(key, value)
}

func (b *Etcd) Delete(key string) error {
	return b.client.Delete(key)
}

func (b *Etcd) List(prefix string) map[string][]byte {
	// TODO impl list in etcd
	return make(map[string][]byte)
}
