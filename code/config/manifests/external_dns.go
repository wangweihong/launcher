package manifests

const (
	externalDnsYaml = `---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: inneruser-external-dns
  namespace: kube-system

---

apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  name: system:external-dns
rules:
- apiGroups:
  - ""
  resources:
  - services
  verbs:
  - get
  - watch
  - list
- apiGroups:
  - extensions
  resources:
  - ingresses
  verbs:
  - get
  - list
  - watch

---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  annotations:
    rbac.authorization.kubernetes.io/autoupdate: "true"
  labels:
    kubernetes.io/bootstrapping: rbac-defaults
  name: system:external-dns
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:external-dns
subjects:
- kind: ServiceAccount
  name: inneruser-external-dns
  namespace: kube-system

---

apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: external-dns
  namespace: kube-system
spec:
  strategy:
    type: Recreate
  template:
    metadata:
      labels:
        app: dev
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: node-role.kubernetes.io/master
                operator: Exists
      serviceAccountName: inneruser-external-dns
      containers:
      - name: external-dns
        image: {{ .ImageExternalDns }}
        args:
        - --source=service
        - --source=ingress
        - --provider=coredns
        - --registry=txt
        - --txt-owner-id=dev.example.org
        - --log-level=debug
        env:
        - name: ETCD_URLS
          value: {{ .EtcdEndpoints }}`
)
