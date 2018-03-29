package manifests

const (
	rbacStorageclassYaml = `apiVersion: v1
kind: ServiceAccount
metadata:
  name: nfs-client-provisioner

---

apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: pvc-admin-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: nfs-client-provisioner
  namespace: default
`
)
