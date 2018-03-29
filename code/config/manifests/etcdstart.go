package manifests

const (
	etcdstartSh = `#!/bin/bash

ufleetNtpName=ufleet-ntp
docker rm -f ${ufleetNtpName} 2>/dev/null || true
docker run -m 500M --memory-reservation 200M -d --privileged=true \
  --restart=always \
  --add-host=ntpd.youruncloud.com:{{ .NtpdHost }} \
  --name=${ufleetNtpName} \
  -v /etc/localtime:/etc/localtime \
  {{ .ImageNtp }}

# wait for ntpd ready. 5 seconds...
sleep 5
docker restart ufleet-ntp || true
docker restart ufleet-ntp || true

docker rm -f infra-{{ .Hostip }} 2>/dev/null || true
docker run -d -m 1G -c 5120 --privileged --name infra-{{ .Hostip }} --restart=always --oom-score-adj=-900 --net="host" \
  -v /var/local/ufleet/launcher/etcd:/var/local/ufleet/launcher/etcd \
  {{ .ImageEtcdAmd64 }}  \
  etcd \
  --name infra-{{ .Hostip }} \
  --auto-compaction-retention=1 \
  --initial-advertise-peer-urls http://{{ .Hostip }}:{{ .PeerPort }} \
  --listen-peer-urls http://{{ .Hostip }}:{{ .PeerPort }} \
  --listen-client-urls http://0.0.0.0:{{ .ListenPort }} \
  --advertise-client-urls http://{{ .Hostip }}:{{ .ListenPort }} \
  --initial-cluster-token {{ .Token }} \
  --initial-cluster {{ .EtcdCluster }} \
  --initial-cluster-state new \
  --data-dir /var/local/ufleet/launcher/etcd/data
`

	etcdstartExistingSh = `#!/bin/bash

ufleetNtpName=ufleet-ntp
docker rm -f ${ufleetNtpName} 2>/dev/null || true
docker run -d -m 500M --memory-reservation 200M --privileged=true \
  --restart=always \
  --add-host=ntpd.youruncloud.com:{{ .NtpdHost }} \
  --name=${ufleetNtpName} \
  -v /etc/localtime:/etc/localtime \
  {{ .ImageNtp }}

# wait for ntpd ready. 5 seconds...
sleep 5
docker restart ufleet-ntp || true
docker restart ufleet-ntp || true

docker rm -f infra-{{ .Hostip }} 2>/dev/null || true
docker run -d -m 1G -c 5120 --privileged=true \
  --name infra-{{ .Hostip }}  \
  --restart=always \
  --net="host" \
  --oom-score-adj=-900 \
  -e ETCD_NAME=infra-{{ .Hostip }} \
  -e ETCD_INITIAL_CLUSTER={{ .EtcdCluster }} \
  -e ETCD_INITIAL_CLUSTER_STATE=existing \
  -v /var/local/ufleet/launcher/etcd:/var/local/ufleet/launcher/etcd \
  {{ .ImageEtcdAmd64 }}  \
  etcd \
  --auto-compaction-retention=1 \
  --initial-advertise-peer-urls http://{{ .Hostip }}:{{ .PeerPort }} \
  --listen-peer-urls http://{{ .Hostip }}:{{ .PeerPort }} \
  --advertise-client-urls http://{{ .Hostip }}:{{ .ListenPort }} \
  --listen-client-urls http://0.0.0.0:{{ .ListenPort }} \
  --data-dir /var/local/ufleet/launcher/etcd/data
`
)
