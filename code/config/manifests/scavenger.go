package manifests

const (
	scavengerYaml = `apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: ufleet-docker-scavenger
  namespace: kube-system
  labels:
    app: ufleet-docker-scavenger
spec:
  template:
    metadata:
      labels:
        app: ufleet-docker-scavenger
    spec:
      hostPID: true
      restartPolicy: Always
      containers:
      - name: ufleet-docker-scavenger
        image: {{ .ImageScavenger }}
        args: [""]
        volumeMounts:
        - name: dockersock
          mountPath: /var/run/docker.sock
          readOnly: false
        - name: varlibdocker
          mountPath: /var/lib/docker
          readOnly: false
      volumes:
      - name: dockersock
        hostPath:
          path: /var/run/docker.sock
      - name: varlibdocker
        hostPath:
          path:  /var/lib/docker
`
)
