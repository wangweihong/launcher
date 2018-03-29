package manifests

const (
	federationAddClusterYaml = `apiVersion: federation/v1beta1
kind: Cluster
metadata:
  name: {{ .ClusterName }}
spec:
  serverAddressByClientCIDRs:
    - clientCIDR: "0.0.0.0/0"
      serverAddress: "{{ .ServiceAddress }}"
  secretRef:
    name: {{ .ClusterSecretName }}`

	corednsProviderConf = `[Global]
etcd-endpoints = {{ .EtcdEndpoints }}
zones = {{ .Zones }}
coredns-endpoints = {{ .CoreDnsEndpoints }}`

	federationValuesYaml = `isClusterService: false
serviceType: "NodePort"
serviceProtocol: "TCP"
middleware:
  kubernetes:
    enabled: false
  etcd:
    enabled: true
    zones:
    - "{{ .Zones }}"
    endpoint: "{{ .EtcdEndpoint }}"
`
	federationInstallerSh = `#!/bin/bash

HOMESCRIPT=$(cd ` + "`" + `dirname $0` + "`" + `;pwd)
export PATH=$PATH:/usr/local/bin

fedinstaller::Install(){
    Log::Register "${FUNCNAME}"

    ok="false"
    for ((i=0;i<60;i++))
    do
        sleep 1
        corednsEndpoints=$(kubectl get endpoints --all-namespaces | grep coredns | head -n 1 | awk '{print $3}' | tr ',' '\n' | tail -n 1)
        [ ${#corednsEndpoints} -eq 0 ] && continue
        [ "x${corednsEndpoints}" == "x<none>" ] && continue
        sed -i "s/{{ .CoreDnsEndpoints }}/${corednsEndpoints}/g" ${HOMESCRIPT}/coredns-provider.conf
        ok="true"
        break
    done

    [ "x${ok}" != "xtrue" ] && echo "can't get coredns endpoint" && exit 1

    kubefed init {{ .FederationName }} \
        --host-cluster-context=kubernetes-admin@kubernetes \
        --dns-provider-config=${HOMESCRIPT}/coredns-provider.conf \
        --dns-provider="coredns" \
        --dns-zone-name="{{ .Zone }}" \
        --api-server-service-type="NodePort" \
        --kubeconfig=/root/.kube/config \
        --etcd-persistent-storage=false \
        --api-server-advertise-address="{{ .Hostip }}"

    Log::UnRegister "${FUNCNAME}"
}
`

	federationHelmRolebindingYaml = `# This role binding allows "dave" to read secrets in the "development" namespace.
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: {{ .ServiceAccountHelm }}-rolebinding
  namespace: kube-system
subjects:
- kind: ServiceAccount
  name: {{ .ServiceAccountHelm }}
  apiGroup: ""
  namespace: kube-system
roleRef:
  kind: ClusterRole
  name: cluster-admin
  apiGroup: rbac.authorization.k8s.io
`
)
