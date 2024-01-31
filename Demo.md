-  I have three users

	* kubeadmin
	* user
	* user1

- I have given role access for `user` to `get, list and patch` the approvaltask

- The role and rolebinding are as follows

```yaml
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: approvaltask-updater
rules:
  - verbs:
      - patch
      - get
      - list
    apiGroups:
      - openshift-pipelines.org
    resources:
      - approvaltasks
```


```yaml
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: user-approvaltask-updater-binding
subjects:
  - kind: User
    apiGroup: rbac.authorization.k8s.io
    name: user
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: approvaltask-updater
```