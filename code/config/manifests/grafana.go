package manifests


const (
	//# grafana/grafana:3.1.1
	grafanaYaml = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: grafana
  namespace: kube-system

---

apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  name: grafana
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: grafana

---
apiVersion: v1
kind: Service
metadata:
  name: grafana
  namespace: kube-system
spec:
  type: NodePort
  ports:
  - port: 3000
    protocol: TCP 
    name: http 
    nodePort: {{ .GrafanaServicePort }}
  selector:
    app: grafana
---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: grafana
  namespace: kube-system
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: grafana
    spec:
      serviceAccountName: grafana
      containers:
      - name: grafana
        image:  {{ .ImageGrafana }}
        imagePullPolicy: IfNotPresent
        ports:
          - containerPort: 3000
        env:
        - name: GRAFANA_PORT
          value: "3000"
        - name: GF_AUTH_BASIC_ENABLED
          value: "false"
        - name: GF_AUTH_ANONYMOUS_ENABLED
          value: "true"
        - name: GF_AUTH_ANONYMOUS_ORG_ROLE
          value: Admin
        - name: GF_PATHS_DATA
          value: /data/grafana
        volumeMounts:
        - mountPath: /data/grafana
          name: grafana-data
      volumes:
      - name: grafana-data
        emptyDir: {}
`
)
