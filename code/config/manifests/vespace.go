package manifests

const (
	vespaceSh = `#!/bin/bash

docker run -d -m 1G --restart=always --net="host" \
  --privileged=true \
  -v /var/local/ufleet/vespace/strategy/lib:/var/lib/vespace \
  -v /var/local/ufleet/vespace/strategy/log:/var/log/vespace \
  -v /dev:/dev \
  -e manager_addr={{ .ManagerAddr }} \
  -e thishostrootpasswd={{ .RootPasswd }} \
  -e etcdname=etcd1 \
  -e etcd1={{ .EtcdIP }} \
  --name etcd1 \
  {{ .ImageVespaceStrategy }}
`

	haVespaceSh = `#!/bin/bash

docker rm -f {{ .EtcdName }} 2>/dev/null || true
docker run -d -m 1G --restart=always --net="host" \
  --privileged=true \
  -v /var/local/ufleet/vespace/strategy/lib:/var/lib/vespace \
  -v /var/local/ufleet/vespace/strategy/log:/var/log/vespace \
  -v /dev/:/dev/ \
  -e etcdname={{ .EtcdName }} \
  -e etcd1={{ .Etcd1IP }} \
  -e etcd2={{ .Etcd2IP }} \
  -e etcd3={{ .Etcd3IP }} \
  -e manager_addr={{ .ManagerAddr }} \
  -e hostrootpasswd={{ .RootPasswd }} \
  --name {{ .EtcdName }} \
  {{ .ImageVespaceHaStrategy }}
`
)
