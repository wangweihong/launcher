package manifests

const (
	keepalivedStorageCmd = `docker run -d --name="ha-vespace-keepalive" --net=host --privileged -e VIRTUAL_IP="{{ .VirtualIP }}" -e VIRTUAL_ROUTER_ID="{{ .VirtualRouterID }}" -e INTERFACE="{{ .Interface }}" -e CONTAINERS_TO_CHECK="{{ .ContainersToCheck }}" -e PRIORITY="99" -e CHECK_FALL="2" -e CHECK_RISE="1" -e CHECK_INTERVAL="2" -e STATE="BACKUP" -v "/var/run/docker.sock":"/var/run/docker.sock" -v "/root/.docker":"/root/.docker" {{ .ImageKeepalived }}`

	rmKeepalivedStorageCmd = `docker rm -f "ha-vespace-keepalive" 2>/dev/null || true`

	keepalivedYaml = `apiVersion: v1
kind: Pod
metadata:
  name: k8s-{{ .KeepalivedName }}-keepalived
  namespace: kube-system
  labels:
    k8s-app: k8s-{{ .KeepalivedName }}-keepalived
spec:
  containers:
  - name: k8s-{{ .KeepalivedName }}-keepalived
    env:
    - name: VIRTUAL_IP
      value: "{{ .VirtualIP }}"
    - name: VIRTUAL_ROUTER_ID
      value: "{{ .VirtualRouterID }}"
    - name: INTERFACE
      value: "{{ .Interface }}"
    - name: PRIORITY
      value: "99"
    - name: CONTAINERS_TO_CHECK
      value: "{{ .ContainersToCheck }}"
    - name: CHECK_FALL
      value: "2"
    - name: CHECK_RISE
      value: "1"
    - name: CHECK_INTERVAL
      value: "2"
    - name: STATE
      value: "BACKUP"
    image: {{ .ImageKeepalived }}
    resources:
      limits:
        memory: 200Mi
      requests:
        cpu: 100m
        memory: 50Mi
    volumeMounts:
    - name: dockersock
      mountPath: "/var/run/docker.sock"
    - name: dockerconfigpath
      mountPath: "/root/.docker"
    securityContext:
      privileged: true
  hostNetwork: true
  terminationGracePeriodSeconds: 30
  volumes:
  - name: dockersock
    hostPath:
      path: "/var/run/docker.sock"
  - name: dockerconfigpath
    hostPath:
      path: "/root/.docker"
`
)
