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

	provisionerYaml=`
apiVersion: v1
kind: ServiceAccount
metadata:
  name: vespace-provisioner
  namespace: kube-system

---

apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  name: vespace-pvc-admin-binding
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: vespace-provisioner
  namespace: kube-system

---
kind: ConfigMap
apiVersion: v1
metadata:
  name: vespace-provisioner-config
  namespace: kube-system
data:
  user: {{ .VespaceUser }}
  password: {{ .VespacePassword }}
  host: {{ .VespaceHost }}


---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: vespace-provisioner
  namespace: kube-system
  labels:
    xfleet-app: vespace-provisioner
spec:
  replicas: 1
  template:
    metadata:
      name: vespace-provisioner
      namespace: kube-system
      labels:
        xfleet-app: vespace-provisioner
    spec:
      containers:
      - name: vespace-provisioner
        command:
        - /deploy/provisioner
        - --user=$(VESPACE_USER)
        - --password=$(VESPACE_PASSWORD)
        - --vespace=$(VESPACE_HOST)
        image: {{ .ImageProvisioner }}
        env:
        - name: VESPACE_USER
          valueFrom:
            configMapKeyRef:
              name: vespace-provisioner-config
              key: user
        - name: VESPACE_PASSWORD
          valueFrom:
            configMapKeyRef:
              name: vespace-provisioner-config
              key: password
        - name: VESPACE_HOST
          valueFrom:
            configMapKeyRef:
              name: vespace-provisioner-config
              key: host
      serviceAccountName: vespace-provisioner
`

)

