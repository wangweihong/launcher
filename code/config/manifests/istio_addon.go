package manifests

const (
	//ImageIstioGrafana: docker.io/istio/grafana:0.7.1
	//ImageIstioPrometheus: docker.io/prom/prometheus:v2.0.0
	//ImageIstioServiceGraph: docker.io/istio/servicegraph:0.7.1
	//ImageIstioZipkinCollector: gcr.io/stackdriver-trace-docker/zipkin-collector
	//ImageIstioZipkin: docker.io/openzipkin/zipkin:latest
	istioAddonYaml = `
---
### grafana
###

---
apiVersion: v1
kind: Service
metadata:
  name: grafana
  namespace: istio-system
spec:
  type: NodePort 
  ports:
  - port: 3000
    protocol: TCP
    name: http
    nodePort: {{ .ServiceIstioGrafanaPort }}
  selector:
    app: grafana
---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: grafana
  namespace: istio-system
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: grafana
      annotations:
        sidecar.istio.io/inject: "false"
    spec:
      serviceAccountName: grafana
      containers:
      - name: grafana
        image: {{ .ImageIstioGrafana }}
        imagePullPolicy: IfNotPresent
        ports:
          - containerPort: 3000
        env:
          # Only put environment related config here. Generic Istio config
          # should go in addons/grafana/grafana.ini.
        - name: GF_PATHS_DATA
          value: /data/grafana
        volumeMounts:
        - mountPath: /data/grafana
          name: grafana-data
      volumes:
      - name: grafana-data
        emptyDir: {}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: grafana
  namespace: istio-system


---
### prometheus
###
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: prometheus
  namespace: istio-system
data:
  prometheus.yml: |-
    global:
      scrape_interval: 15s
    scrape_configs:

    - job_name: 'istio-mesh'
      # Override the global default and scrape targets from this job every 5 seconds.
      scrape_interval: 5s

      kubernetes_sd_configs:
      - role: endpoints

      relabel_configs:
      - source_labels: [__meta_kubernetes_namespace, __meta_kubernetes_service_name, __meta_kubernetes_endpoint_port_name]
        action: keep
        regex: istio-system;istio-mixer;prometheus

    - job_name: 'envoy'
      # Override the global default and scrape targets from this job every 5 seconds.
      scrape_interval: 5s
      # metrics_path defaults to '/metrics'
      # scheme defaults to 'http'.

      kubernetes_sd_configs:
      - role: endpoints

      relabel_configs:
      - source_labels: [__meta_kubernetes_namespace, __meta_kubernetes_service_name, __meta_kubernetes_endpoint_port_name]
        action: keep
        regex: istio-system;istio-mixer;statsd-prom

    - job_name: 'mixer'
      # Override the global default and scrape targets from this job every 5 seconds.
      scrape_interval: 5s
      # metrics_path defaults to '/metrics'
      # scheme defaults to 'http'.

      kubernetes_sd_configs:
      - role: endpoints

      relabel_configs:
      - source_labels: [__meta_kubernetes_namespace, __meta_kubernetes_service_name, __meta_kubernetes_endpoint_port_name]
        action: keep
        regex: istio-system;istio-mixer;http-monitoring

    - job_name: 'pilot'
      # Override the global default and scrape targets from this job every 5 seconds.
      scrape_interval: 5s
      # metrics_path defaults to '/metrics'
      # scheme defaults to 'http'.

      kubernetes_sd_configs:
      - role: endpoints

      relabel_configs:
      - source_labels: [__meta_kubernetes_namespace, __meta_kubernetes_service_name, __meta_kubernetes_endpoint_port_name]
        action: keep
        regex: istio-system;istio-pilot;http-monitoring

    # scrape config for API servers
    - job_name: 'kubernetes-apiservers'
      kubernetes_sd_configs:
      - role: endpoints
      scheme: https
      tls_config:
        ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
      bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token
      relabel_configs:
      - source_labels: [__meta_kubernetes_namespace, __meta_kubernetes_service_name, __meta_kubernetes_endpoint_port_name]
        action: keep
        regex: default;kubernetes;https

    # scrape config for nodes (kubelet)
    - job_name: 'kubernetes-nodes'
      scheme: https
      tls_config:
        ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
      bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token
      kubernetes_sd_configs:
      - role: node
      relabel_configs:
      - action: labelmap
        regex: __meta_kubernetes_node_label_(.+)
      - target_label: __address__
        replacement: kubernetes.default.svc:443
      - source_labels: [__meta_kubernetes_node_name]
        regex: (.+)
        target_label: __metrics_path__
        replacement: /api/v1/nodes/${1}/proxy/metrics

    # Scrape config for Kubelet cAdvisor.
    #
    # This is required for Kubernetes 1.7.3 and later, where cAdvisor metrics
    # (those whose names begin with 'container_') have been removed from the
    # Kubelet metrics endpoint.  This job scrapes the cAdvisor endpoint to
    # retrieve those metrics.
    #
    # In Kubernetes 1.7.0-1.7.2, these metrics are only exposed on the cAdvisor
    # HTTP endpoint; use "replacement: /api/v1/nodes/${1}:4194/proxy/metrics"
    # in that case (and ensure cAdvisor's HTTP server hasn't been disabled with
    # the --cadvisor-port=0 Kubelet flag).
    #
    # This job is not necessary and should be removed in Kubernetes 1.6 and
    # earlier versions, or it will cause the metrics to be scraped twice.
    - job_name: 'kubernetes-cadvisor'
      scheme: https
      tls_config:
        ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
      bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token
      kubernetes_sd_configs:
      - role: node
      relabel_configs:
      - action: labelmap
        regex: __meta_kubernetes_node_label_(.+)
      - target_label: __address__
        replacement: kubernetes.default.svc:443
      - source_labels: [__meta_kubernetes_node_name]
        regex: (.+)
        target_label: __metrics_path__
        replacement: /api/v1/nodes/${1}/proxy/metrics/cadvisor

    # scrape config for service endpoints.
    - job_name: 'kubernetes-service-endpoints'
      kubernetes_sd_configs:
      - role: endpoints
      relabel_configs:
      - source_labels: [__meta_kubernetes_service_annotation_prometheus_io_scrape]
        action: keep
        regex: true
      - source_labels: [__meta_kubernetes_service_annotation_prometheus_io_scheme]
        action: replace
        target_label: __scheme__
        regex: (https?)
      - source_labels: [__meta_kubernetes_service_annotation_prometheus_io_path]
        action: replace
        target_label: __metrics_path__
        regex: (.+)
      - source_labels: [__address__, __meta_kubernetes_service_annotation_prometheus_io_port]
        action: replace
        target_label: __address__
        regex: ([^:]+)(?::\d+)?;(\d+)
        replacement: $1:$2
      - action: labelmap
        regex: __meta_kubernetes_service_label_(.+)
      - source_labels: [__meta_kubernetes_namespace]
        action: replace
        target_label: kubernetes_namespace
      - source_labels: [__meta_kubernetes_service_name]
        action: replace
        target_label: kubernetes_name

    # Example scrape config for pods
    - job_name: 'kubernetes-pods'
      kubernetes_sd_configs:
      - role: pod

      relabel_configs:
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
        action: keep
        regex: true
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_path]
        action: replace
        target_label: __metrics_path__
        regex: (.+)
      - source_labels: [__address__, __meta_kubernetes_pod_annotation_prometheus_io_port]
        action: replace
        regex: ([^:]+)(?::\d+)?;(\d+)
        replacement: $1:$2
        target_label: __address__
      - action: labelmap
        regex: __meta_kubernetes_pod_label_(.+)
      - source_labels: [__meta_kubernetes_namespace]
        action: replace
        target_label: namespace
      - source_labels: [__meta_kubernetes_pod_name]
        action: replace
        target_label: pod_name

---
apiVersion: v1
kind: Service
metadata:
  annotations:
    prometheus.io/scrape: 'true'
  labels:
    name: prometheus
  name: prometheus
  namespace: istio-system
spec:
  selector:
    app: prometheus
  ports:
  - name: prometheus
    protocol: TCP
    port: 9090
---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: prometheus
  namespace: istio-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: prometheus
  template:
    metadata:
      name: prometheus
      labels:
        app: prometheus
      annotations:
        sidecar.istio.io/inject: "false"
    spec:
      serviceAccountName: prometheus
      containers:
      - name: prometheus
        image: {{ .ImageIstioPrometheus }}
        imagePullPolicy: IfNotPresent
        args:
          - '--storage.tsdb.retention=6h'
          - '--config.file=/etc/prometheus/prometheus.yml'
        ports:
        - name: web
          containerPort: 9090
        volumeMounts:
        - name: config-volume
          mountPath: /etc/prometheus
      volumes:
      - name: config-volume
        configMap:
          name: prometheus
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: prometheus
  namespace: istio-system
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  name: prometheus
rules:
- apiGroups: [""]
  resources:
  - nodes
  - services
  - endpoints
  - pods
  - nodes/proxy
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources:
  - configmaps
  verbs: ["get"]
- nonResourceURLs: ["/metrics"]
  verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  name: prometheus
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: prometheus
subjects:
- kind: ServiceAccount
  name: prometheus
  namespace: istio-system

---
### service graph
###
---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: servicegraph
  namespace: istio-system
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: servicegraph
      annotations:
        sidecar.istio.io/inject: "false"
    spec:
      containers:
      - name: servicegraph
        image: {{ .ImageIstioServiceGraph }}
        imagePullPolicy: IfNotPresent
        ports:
          - containerPort: 8088
        args:
        - --prometheusAddr=http://prometheus:9090
---
apiVersion: v1
kind: Service
metadata:a
  name: servicegraph
  namespace: istio-system
spec:
  ports:
  - name: http
    port: 8088
  selector:
    app: servicegraph

---

### zipkin-to-stack
###
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: zipkin-to-stackdriver
  namespace: istio-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: zipkin-to-stackdriver
  template:
    metadata:
      name: zipkin-to-stackdriver
      labels:
        app: zipkin-to-stackdriver
      annotations:
        sidecar.istio.io/inject: "false"
    spec:
      containers:
      - name: zipkin-to-stackdriver
        image: {{ .ImageIstioZipkinCollector }}
        imagePullPolicy: IfNotPresent
#        env:
#        - name: GOOGLE_APPLICATION_CREDENTIALS
#          value: "/path/to/credentials.json"
#        - name: PROJECT_ID
#          value: "my_project_id"
        ports:
        - name: zipkin
          containerPort: 9411
---
apiVersion: v1
kind: Service
metadata:
  name: zipkin-to-stackdriver
spec:
  ports:
  - name: zipkin
    port: 9411
  selector:
    app: zipkin-to-stackdriver

---
### zipkin
###

---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: zipkin
  namespace: istio-system
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: zipkin
      annotations:
        sidecar.istio.io/inject: "false"
    spec:
      containers:
      - name: zipkin
        image: {{ .ImageIstioZipkin }}
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 9411
        env:
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.namespace
---
apiVersion: v1
kind: Service
metadata:
  name: zipkin
  namespace: istio-system
spec:
  ports:
  - name: http
    port: 9411
  selector:
    app: zipkin
---

`
)
