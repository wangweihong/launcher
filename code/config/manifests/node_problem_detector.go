package manifests

const (
	//k8s.gcr.io/node-problem-detector:v0.4.1
	nodeProblemDetectorYaml=`
apiVersion: v1
data:
  abrt-adaptor.json: "{\n\t\"plugin\": \"journald\",\n\t\"pluginConfig\": {\n\t\t\"source\":
    \"abrt-notification\"\n\t},\n\t\"logPath\": \"/var/log/journal\",\n\t\"lookback\":
    \"5m\",\n\t\"bufferSize\": 10,\n\t\"source\": \"abrt-adaptor\",\n\t\"conditions\":
    [],\n\t\"rules\": [\n\t\t{\n\t\t\t\"type\": \"temporary\",\n\t\t\t\"reason\":
    \"CCPPCrash\",\n\t\t\t\"pattern\": \"Process \\\\d+ \\\\(\\\\S+\\\\) crashed in
    .*\"\n\t\t},\n\t\t{\n\t\t\t\"type\": \"temporary\",\n\t\t\t\"reason\": \"UncaughtException\",\n\t\t\t\"pattern\":
    \"Process \\\\d+ \\\\(\\\\S+\\\\) of user \\\\d+ encountered an uncaught \\\\S+
    exception\"\n\t\t},\n\t\t{\n\t\t\t\"type\": \"temporary\",\n\t\t\t\"reason\":
    \"XorgCrash\",\n\t\t\t\"pattern\": \"Display server \\\\S+ crash in \\\\S+\"\n\t\t},\n\t\t{\n\t\t\t\"type\":
    \"temporary\",\n\t\t\t\"reason\": \"VMcore\",\n\t\t\t\"pattern\": \"System encountered
    a fatal error in \\\\S+\"\n\t\t},\n\t\t{\n\t\t\t\"type\": \"temporary\",\n\t\t\t\"reason\":
    \"Kerneloops\",\n\t\t\t\"pattern\": \"System encountered a non-fatal error in
    \\\\S+\"\n\t\t}\n\t]\n}\n"
  docker-monitor-filelog.json: "{\n\t\"plugin\": \"filelog\",\n\t\"pluginConfig\":
    {\n\t\t\"timestamp\": \"^time=\\\"(\\\\S*)\\\"\",\n\t\t\"message\": \"msg=\\\"([^\\n]*)\\\"\",\n\t\t\"timestampFormat\":
    \"2006-01-02T15:04:05.999999999-07:00\"\n\t},\n\t\"logPath\": \"/var/log/docker.log\",\n\t\"lookback\":
    \"5m\",\n\t\"bufferSize\": 10,\n\t\"source\": \"docker-monitor\",\n\t\"conditions\":
    [],\n\t\"rules\": [\n\t\t{\n\t\t\t\"type\": \"temporary\",\n\t\t\t\"reason\":
    \"CorruptDockerImage\",\n\t\t\t\"pattern\": \"Error trying v2 registry: failed
    to register layer: rename /var/lib/docker/image/(.+) /var/lib/docker/image/(.+):
    directory not empty.*\"\n\t\t}\n\t]\n}\n"
  docker-monitor.json: "{\n\t\"plugin\": \"journald\",\n\t\"pluginConfig\": {\n\t\t\"source\":
    \"docker\"\n\t},\n\t\"logPath\": \"/var/log/journal\",\n\t\"lookback\": \"5m\",\n\t\"bufferSize\":
    10,\n\t\"source\": \"docker-monitor\",\n\t\"conditions\": [],\n\t\"rules\": [\n\t\t{\n\t\t\t\"type\":
    \"temporary\",\n\t\t\t\"reason\": \"CorruptDockerImage\",\n\t\t\t\"pattern\":
    \"Error trying v2 registry: failed to register layer: rename /var/lib/docker/image/(.+)
    /var/lib/docker/image/(.+): directory not empty.*\"\n\t\t}\n\t]\n}\n"
  kernel-monitor-filelog.json: "{\n\t\"plugin\": \"filelog\",\n\t\"pluginConfig\":
    {\n\t\t\"timestamp\": \"^.{15}\",\n\t\t\"message\": \"kernel: \\\\[.*\\\\] (.*)\",\n\t\t\"timestampFormat\":
    \"Jan _2 15:04:05\"\n\t},\n\t\"logPath\": \"/var/log/kern.log\",\n\t\"lookback\":
    \"5m\",\n\t\"bufferSize\": 10,\n\t\"source\": \"kernel-monitor\",\n\t\"conditions\":
    [\n\t\t{\n\t\t\t\"type\": \"KernelDeadlock\",\n\t\t\t\"reason\": \"KernelHasNoDeadlock\",\n\t\t\t\"message\":
    \"kernel has no deadlock\"\n\t\t}\n\t],\n\t\"rules\": [\n\t\t{\n\t\t\t\"type\":
    \"temporary\",\n\t\t\t\"reason\": \"OOMKilling\",\n\t\t\t\"pattern\": \"Kill process
    \\\\d+ (.+) score \\\\d+ or sacrifice child\\\\nKilled process \\\\d+ (.+) total-vm:\\\\d+kB,
    anon-rss:\\\\d+kB, file-rss:\\\\d+kB\"\n\t\t},\n\t\t{\n\t\t\t\"type\": \"temporary\",\n\t\t\t\"reason\":
    \"TaskHung\",\n\t\t\t\"pattern\": \"task \\\\S+:\\\\w+ blocked for more than \\\\w+
    seconds\\\\.\"\n\t\t},\n\t\t{\n\t\t\t\"type\": \"temporary\",\n\t\t\t\"reason\":
    \"UnregisterNetDevice\",\n\t\t\t\"pattern\": \"unregister_netdevice: waiting for
    \\\\w+ to become free. Usage count = \\\\d+\"\n\t\t},\n\t\t{\n\t\t\t\"type\":
    \"temporary\",\n\t\t\t\"reason\": \"KernelOops\",\n\t\t\t\"pattern\": \"BUG: unable
    to handle kernel NULL pointer dereference at .*\"\n\t\t},\n\t\t{\n\t\t\t\"type\":
    \"temporary\",\n\t\t\t\"reason\": \"KernelOops\",\n\t\t\t\"pattern\": \"divide
    error: 0000 \\\\[#\\\\d+\\\\] SMP\"\n\t\t},\n\t\t{\n\t\t\t\"type\": \"permanent\",\n\t\t\t\"condition\":
    \"KernelDeadlock\",\n\t\t\t\"reason\": \"AUFSUmountHung\",\n\t\t\t\"pattern\":
    \"task umount\\\\.aufs:\\\\w+ blocked for more than \\\\w+ seconds\\\\.\"\n\t\t},\n\t\t{\n\t\t\t\"type\":
    \"permanent\",\n\t\t\t\"condition\": \"KernelDeadlock\",\n\t\t\t\"reason\": \"DockerHung\",\n\t\t\t\"pattern\":
    \"task docker:\\\\w+ blocked for more than \\\\w+ seconds\\\\.\"\n\t\t}\n\t]\n}\n"
  kernel-monitor.json: "{\n\t\"plugin\": \"journald\",\n\t\"pluginConfig\": {\n\t\t\"source\":
    \"kernel\"\n\t},\n\t\"logPath\": \"/var/log/journal\",\n\t\"lookback\": \"5m\",\n\t\"bufferSize\":
    10,\n\t\"source\": \"kernel-monitor\",\n\t\"conditions\": [\n\t\t{\n\t\t\t\"type\":
    \"KernelDeadlock\",\n\t\t\t\"reason\": \"KernelHasNoDeadlock\",\n\t\t\t\"message\":
    \"kernel has no deadlock\"\n\t\t}\n\t],\n\t\"rules\": [\n\t\t{\n\t\t\t\"type\":
    \"temporary\",\n\t\t\t\"reason\": \"OOMKilling\",\n\t\t\t\"pattern\": \"Kill process
    \\\\d+ (.+) score \\\\d+ or sacrifice child\\\\nKilled process \\\\d+ (.+) total-vm:\\\\d+kB,
    anon-rss:\\\\d+kB, file-rss:\\\\d+kB\"\n\t\t},\n\t\t{\n\t\t\t\"type\": \"temporary\",\n\t\t\t\"reason\":
    \"TaskHung\",\n\t\t\t\"pattern\": \"task \\\\S+:\\\\w+ blocked for more than \\\\w+
    seconds\\\\.\"\n\t\t},\n\t\t{\n\t\t\t\"type\": \"temporary\",\n\t\t\t\"reason\":
    \"UnregisterNetDevice\",\n\t\t\t\"pattern\": \"unregister_netdevice: waiting for
    \\\\w+ to become free. Usage count = \\\\d+\"\n\t\t},\n\t\t{\n\t\t\t\"type\":
    \"temporary\",\n\t\t\t\"reason\": \"KernelOops\",\n\t\t\t\"pattern\": \"BUG: unable
    to handle kernel NULL pointer dereference at .*\"\n\t\t},\n\t\t{\n\t\t\t\"type\":
    \"temporary\",\n\t\t\t\"reason\": \"KernelOops\",\n\t\t\t\"pattern\": \"divide
    error: 0000 \\\\[#\\\\d+\\\\] SMP\"\n\t\t},\n\t\t{\n\t\t\t\"type\": \"permanent\",\n\t\t\t\"condition\":
    \"KernelDeadlock\",\n\t\t\t\"reason\": \"AUFSUmountHung\",\n\t\t\t\"pattern\":
    \"task umount\\\\.aufs:\\\\w+ blocked for more than \\\\w+ seconds\\\\.\"\n\t\t},\n\t\t{\n\t\t\t\"type\":
    \"permanent\",\n\t\t\t\"condition\": \"KernelDeadlock\",\n\t\t\t\"reason\": \"DockerHung\",\n\t\t\t\"pattern\":
    \"task docker:\\\\w+ blocked for more than \\\\w+ seconds\\\\.\"\n\t\t}\n\t]\n}\n"
kind: ConfigMap
metadata:
  name:  node-problem-detector-config
  namespace: kube-system
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: node-problem-detector
  namespace: kube-system
  labels:
    kubernetes.io/cluster-service: "true"
    addonmanager.kubernetes.io/mode: Reconcile
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: npd-binding
  labels:
    kubernetes.io/cluster-service: "true"
    addonmanager.kubernetes.io/mode: Reconcile
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:node-problem-detector
subjects:
- kind: ServiceAccount
  name: node-problem-detector
  namespace: kube-system
---
apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: npd-v0.4.1
  namespace: kube-system
  labels:
    k8s-app: node-problem-detector
    version: v0.4.1
    kubernetes.io/cluster-service: "true"
    addonmanager.kubernetes.io/mode: Reconcile
spec:
  template:
    metadata:
      labels:
        k8s-app: node-problem-detector
        version: v0.4.1
        kubernetes.io/cluster-service: "true"
    spec:
      containers:
      - name: node-problem-detector
        image: k8s.gcr.io/node-problem-detector:v0.4.1
        command:
        - "/bin/sh"
        - "-c"
        # Pass both config to support both journald and syslog.
        - "exec /node-problem-detector --logtostderr --system-log-monitors=/config/kernel-monitor.json,/config/kernel-monitor-filelog.json,/config/docker-monitor.json,/config/docker-monitor-filelog.json >>/var/log/node-problem-detector.log 2>&1"
        securityContext:
          privileged: true
        resources:
          limits:
            cpu: "200m"
            memory: "100Mi"
          requests:
            cpu: "20m"
            memory: "20Mi"
        livenessProbe:
          httpGet:
            path: "/healthz"
            host: "127.0.0.1"
            port: 10256
          initialDelaySeconds: 5
          periodSeconds: 5
        env:
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        volumeMounts:
        - name: log
          mountPath: /var/log
        - name: localtime
          mountPath: /etc/localtime
          readOnly: true
        - name: config 
          mountPath: /config
          readOnly: true
      volumes:
      - name: log
        hostPath:
          path: /var/log/
      - name: localtime
        hostPath:
          path: /etc/localtime
          type: "FileOrCreate"
      - name: config # Define ConfigMap volume
        configMap:
          name: node-problem-detector-config
      serviceAccountName: node-problem-detector
      tolerations:
      - operator: "Exists"
        effect: "NoExecute"
      - key: "CriticalAddonsOnly"
        operator: "Exists"
`
)
