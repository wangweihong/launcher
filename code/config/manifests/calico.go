package manifests

const(
	calicoYaml = `# Calico Version v2.5.1
# https://docs.projectcalico.org/v2.5/releases#v2.5.1
# This manifest includes the following component versions:
#   calico/node:v2.5.1
#   calico/cni:v1.10.0
#   calico/kube-policy-controller:v0.7.0

# This ConfigMap is used to configure a self-hosted Calico installation.
kind: ConfigMap
apiVersion: v1
metadata:
  name: calico-config
  namespace: kube-system
data:
  # The location of your etcd cluster.  This uses the Service clusterIP
  # defined below.
  etcd_endpoints: "{{ .CalicoEtcdCluster }}"

  # Configure the Calico backend to use.
  calico_backend: "bird"

  # The CNI network configuration to install on each node.
  cni_network_config: |-
    {
        "name": "k8s-pod-network",
        "cniVersion": "0.1.0",
        "type": "calico",
        "etcd_endpoints": "__ETCD_ENDPOINTS__",
        "log_level": "info",
        "mtu": 1500,
        "ipam": {
            "type": "calico-ipam"
        },
        "policy": {
            "type": "k8s",
             "k8s_api_root": "https://__KUBERNETES_SERVICE_HOST__:__KUBERNETES_SERVICE_PORT__",
             "k8s_auth_token": "__SERVICEACCOUNT_TOKEN__"
        },
        "kubernetes": {
            "kubeconfig": "/etc/cni/net.d/__KUBECONFIG_FILENAME__"
        }
    }

---

# This manifest installs the calico/node container, as well
# as the Calico CNI plugins and network config on
# each master and worker node in a Kubernetes cluster.
kind: DaemonSet
apiVersion: extensions/v1beta1
metadata:
  name: calico-node
  namespace: kube-system
  labels:
    k8s-app: calico-node
spec:
  selector:
    matchLabels:
      k8s-app: calico-node
  template:
    metadata:
      labels:
        k8s-app: calico-node
      annotations:
        # Mark this pod as a critical add-on; when enabled, the critical add-on scheduler
        # reserves resources for critical add-on pods so that they can be rescheduled after
        # a failure.  This annotation works in tandem with the toleration below.
        scheduler.alpha.kubernetes.io/critical-pod: ''
    spec:
      hostNetwork: true
      tolerations:
      # Modify!! NoSchedule will be remove for allowing schedule pods on master node.
      - key: node-role.kubernetes.io/master
        operator: Exists
      # Allow this pod to be rescheduled while the node is in "critical add-ons only" mode.
      # This, along with the annotation above marks this pod as a critical add-on.
      - key: CriticalAddonsOnly
        operator: Exists
      serviceAccountName: calico-cni-plugin
      containers:
        # Runs calico/node container on each Kubernetes node.  This
        # container programs network policy and routes on each
        # host.
        - name: calico-node
          image: {{ .ImageCalicoNode }}
          env:
            # The location of the Calico etcd cluster.
            - name: ETCD_ENDPOINTS
              valueFrom:
                configMapKeyRef:
                  name: calico-config
                  key: etcd_endpoints
            # Enable BGP.  Disable to enforce policy only.
            - name: CALICO_NETWORKING_BACKEND
              valueFrom:
                configMapKeyRef:
                  name: calico-config
                  key: calico_backend
            # Cluster type to identify the deployment type
            - name: CLUSTER_TYPE
              value: "kubeadm,bgp"
            # Disable file logging so kubectl logs works.
            - name: CALICO_DISABLE_FILE_LOGGING
              value: "true"
            # Set Felix endpoint to host default action to ACCEPT.
            - name: FELIX_DEFAULTENDPOINTTOHOSTACTION
              value: "ACCEPT"
            # Configure the IP Pool from which Pod IPs will be chosen.
            - name: CALICO_IPV4POOL_CIDR
              value: "{{ .DefaultNetPodSubnet }}"
            - name: CALICO_IPV4POOL_IPIP
              value: "always"
            # Disable IPv6 on Kubernetes.
            - name: FELIX_IPV6SUPPORT
              value: "false"
            # Set MTU for tunnel device used if ipip is enabled
            - name: FELIX_IPINIPMTU
              value: "1440"
            # Set Felix logging to "info"
            - name: FELIX_LOGSEVERITYSCREEN
              value: "info"
            # Auto-detect the BGP IP address.
            - name: IP
              value: ""
            - name: FELIX_HEALTHENABLED
              value: "true"
            - name: CALICO_LIBNETWORK_CREATE_PROFILES
              value: "true"
          securityContext:
            privileged: true
          resources:
            requests:
              cpu: 250m
          livenessProbe:
            httpGet:
              path: /liveness
              port: 9099
            periodSeconds: 10
            initialDelaySeconds: 10
            failureThreshold: 6
          readinessProbe:
            httpGet:
              path: /readiness
              port: 9099
            periodSeconds: 10
          volumeMounts:
            - mountPath: /lib/modules
              name: lib-modules
              readOnly: true
            - mountPath: /var/run/calico
              name: var-run-calico
              readOnly: false
        # This container installs the Calico CNI binaries
        # and CNI network config file on each node.
        - name: install-cni
          image: {{ .ImageCalicoCni }}
          command: ["/install-cni.sh"]
          env:
            # The location of the Calico etcd cluster.
            - name: ETCD_ENDPOINTS
              valueFrom:
                configMapKeyRef:
                  name: calico-config
                  key: etcd_endpoints
            # The CNI network config to install on each node.
            - name: CNI_NETWORK_CONFIG
              valueFrom:
                configMapKeyRef:
                  name: calico-config
                  key: cni_network_config
          volumeMounts:
            - mountPath: /host/opt/cni/bin
              name: cni-bin-dir
            - mountPath: /host/etc/cni/net.d
              name: cni-net-dir
      volumes:
        # Used by calico/node.
        - name: lib-modules
          hostPath:
            path: /lib/modules
        - name: var-run-calico
          hostPath:
            path: /var/run/calico
        # Used to install CNI.
        - name: cni-bin-dir
          hostPath:
            path: /opt/cni/bin
        - name: cni-net-dir
          hostPath:
            path: /etc/cni/net.d

---

# This manifest deploys the Calico policy controller on Kubernetes.
# See https://github.com/projectcalico/k8s-policy
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: calico-policy-controller
  namespace: kube-system
  labels:
    k8s-app: calico-policy
spec:
  # The policy controller can only have a single active instance.
  replicas: 1
  strategy:
    type: Recreate
  template:
    metadata:
      name: calico-policy-controller
      namespace: kube-system
      labels:
        k8s-app: calico-policy-controller
      annotations:
        # Mark this pod as a critical add-on; when enabled, the critical add-on scheduler
        # reserves resources for critical add-on pods so that they can be rescheduled after
        # a failure.  This annotation works in tandem with the toleration below.
        scheduler.alpha.kubernetes.io/critical-pod: ''
    spec:
      # The policy controller must run in the host network namespace so that
      # it isn't governed by policy that would prevent it from working.
      hostNetwork: true
      tolerations:
      # Modify!! NoSchedule will be remove for allowing schedule pods on master node.
      - key: node-role.kubernetes.io/master
        operator: Exists
      # Allow this pod to be rescheduled while the node is in "critical add-ons only" mode.
      # This, along with the annotation above marks this pod as a critical add-on.
      - key: CriticalAddonsOnly
        operator: Exists
      serviceAccountName: calico-policy-controller
      containers:
        - name: calico-policy-controller
          image: {{ .ImageCalicoKubePolicyController }}
          env:
            # The location of the Calico etcd cluster.
            - name: ETCD_ENDPOINTS
              valueFrom:
                configMapKeyRef:
                  name: calico-config
                  key: etcd_endpoints
            # The location of the Kubernetes API.  Use the default Kubernetes
            # service for API access.
            - name: K8S_API
              value: "https://kubernetes.default:443"
            # Since we're running in the host namespace and might not have KubeDNS
            # access, configure the container's /etc/hosts to resolve
            # kubernetes.default to the correct service clusterIP.
            - name: CONFIGURE_ETC_HOSTS
              value: "true"
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  name: calico-cni-plugin
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: calico-cni-plugin
subjects:
- kind: ServiceAccount
  name: calico-cni-plugin
  namespace: kube-system
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: calico-cni-plugin
  namespace: kube-system
rules:
  - apiGroups: [""]
    resources:
      - pods
      - nodes
    verbs:
      - get
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: calico-cni-plugin
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  name: calico-policy-controller
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: calico-policy-controller
subjects:
- kind: ServiceAccount
  name: calico-policy-controller
  namespace: kube-system
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: calico-policy-controller
  namespace: kube-system
rules:
  - apiGroups:
    - ""
    - extensions
    resources:
      - pods
      - namespaces
      - networkpolicies
    verbs:
      - watch
      - list
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: calico-policy-controller
  namespace: kube-system
`
)