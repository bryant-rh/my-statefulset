# 背景

通过kubebuilder 实现一个类似statefulset controller 包含AdmissionWebhook

步骤如下:

```Bash
# 1.初始化
kubebuilder init --domain mystatefulset.com --repo github.com/bryant-rh/my-statefulset --owner "bryant-rh"

# 2. 创建api
kubebuilder create api --group apps --version v1 --kind MyStatefulset

# 3. 创建AdmissionWebhook
kubebuilder create webhook --group apps --version v1 --kind MyStatefulset --defaulting --programmatic-validation

# 本地部署测试
make generate
make manifests

# 部署crd
make install
# 运行
make run

```

# deploy

```bash
# 会自动把kustomize 生成的文件转换成helm包并进行部署
make helm-install

# 部署资源进行测试
# 注意部署文件中replicas=0,为了测试AdmissionWebhook 是否能检测到replicas=0 的时候，会赋值默认值=1
kubectl apply -f config/samples/apps_v1_mystatefulset.yaml
```

# 效果

```bash
# 部署
$ kubectl apply -f config/samples/apps_v1_mystatefulset.yaml
mystatefulset.apps.mystatefulset.com/mystatefulset-sample created

$ kubectl get pods
NAME                     READY   STATUS    RESTARTS   AGE
mystatefulset-sample-0   1/1     Running   0          7s

# 扩容
$ kubectl scale --replicas=2 kms/mystatefulset-sample
mystatefulset.apps.mystatefulset.com/mystatefulset-sample scaled

$ kubectl get pods
NAME                     READY   STATUS    RESTARTS   AGE
mystatefulset-sample-0   1/1     Running   0          40s
mystatefulset-sample-1   1/1     Running   0          11s

# 缩容
$ kubectl scale --replicas=1 kms/mystatefulset-sample
mystatefulset.apps.mystatefulset.com/mystatefulset-sample scaled

$ kubectl get pods
NAME                     READY   STATUS    RESTARTS   AGE
mystatefulset-sample-0   1/1     Running   0          62s

$ kubectl apply -f config/samples/apps_v1_mystatefulset.yaml

$ kubectl scale --replicas=0 kms/mystatefulset-sample
mystatefulset.apps.mystatefulset.com/mystatefulset-sample scaled

$ kubectl get pods
NAME                     READY   STATUS    RESTARTS   AGE
mystatefulset-sample-0   1/1     Running   0          7m54s

$ kubectl get pvc
NAME                         STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS   AGE
www-mystatefulset-sample-0   Bound    pvc-6b1def68-e9d5-4d8c-b627-496a5d844332   1Gi        RWO            local-path     14m
www-mystatefulset-sample-1   Bound    pvc-d56294d0-c1ae-416f-a228-e77e15d5e322   1Gi        RWO            local-path     14m
```

# 单元测试

```Bash
# 运行单元测试
make test-unit

# 查看测试覆盖率
make test-coverage

# 运行特定测试
make test-specific TEST_PATTERN=TestMyStatefulsetReconciler

```
