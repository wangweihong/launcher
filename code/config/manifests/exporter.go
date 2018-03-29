package manifests

const (
	exporterYaml = `apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: prometheus-node-exporter
  namespace: kube-system
  labels:
    tier: node
    app: prometheus-node-exporter
spec:
  template:
    metadata:
      labels:
        tier: node
        app: prometheus-node-exporter
    spec:
      hostNetwork: true
      hostPID: true
      nodeSelector:
        beta.kubernetes.io/arch: amd64
      containers:
      - name: prometheus-node-exporter
        image: {{ .ImagePrometheusNodeExporter }}
        args: ["-collector.procfs", "/host/proc", "-collector.sysfs", "/host/sys", "-collector.filesystem.ignored-mount-points", "^/(sys|proc|dev|host|etc)($|/)", "-collectors.enabled", "diskstats,filefd,filesystem,loadavg,meminfo,netdev,stat,time,uname,vmstat"]
        ports:
        - containerPort: 9100
          hostPort: 9100
          protocol: TCP
        volumeMounts:
        - name: proc
          mountPath: /host/proc
          readOnly: true
        - name: sys
          mountPath: /host/sys
          readOnly: true
        - name: rootfs
          readOnly: true
          mountPath: /rootfs
      volumes:
      - name: proc
        hostPath:
          path: /proc
      - name: sys
        hostPath:
          path: /sys
      - name: rootfs
        hostPath:
          path: /

`
)
