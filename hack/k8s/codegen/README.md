## How to use codegen

```sh
./hack/k8s/codegen/update-generated.sh
```

It should print:

```
Generating deepcopy funcs
Generating clientset for etcd:v1beta2 at github.com/coreos/etcd-operator/pkg/generated/clientset
Generating listers for etcd:v1beta2 at github.com/coreos/etcd-operator/pkg/generated/listers
Generating informers for etcd:v1beta2 at github.com/coreos/etcd-operator/pkg/generated/informers
```
