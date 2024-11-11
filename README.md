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

## 单元测试结果
```Bash
$ make test-unit 

go fmt ./...
go vet ./...
go test ./... -v -coverprofile cover.out
?   	github.com/bryant-rh/my-statefulset	[no test files]
=== RUN   TestMyStatefulset_ValidateCreate
=== RUN   TestMyStatefulset_ValidateCreate/valid_replicas
=== RUN   TestMyStatefulset_ValidateCreate/invalid_replicas
--- PASS: TestMyStatefulset_ValidateCreate (0.00s)
    --- PASS: TestMyStatefulset_ValidateCreate/valid_replicas (0.00s)
    --- PASS: TestMyStatefulset_ValidateCreate/invalid_replicas (0.00s)
=== RUN   TestMyStatefulset_ValidateUpdate
=== RUN   TestMyStatefulset_ValidateUpdate/valid_update
--- PASS: TestMyStatefulset_ValidateUpdate (0.00s)
    --- PASS: TestMyStatefulset_ValidateUpdate/valid_update (0.00s)
=== RUN   TestAPIs
    webhook_suite_test.go:54: Skipping integration tests
--- SKIP: TestAPIs (0.00s)
PASS
coverage: 26.3% of statements
ok  	github.com/bryant-rh/my-statefulset/api/v1	3.346s	coverage: 26.3% of statements
=== RUN   TestMyStatefulsetReconciler_Reconcile
=== RUN   TestMyStatefulsetReconciler_Reconcile/Valid_MyStatefulset
=== RUN   TestMyStatefulsetReconciler_Reconcile/Scale_Up_Scenario
=== RUN   TestMyStatefulsetReconciler_Reconcile/Scale_Down_Scenario
=== RUN   TestMyStatefulsetReconciler_Reconcile/Update_Pod_Template
--- PASS: TestMyStatefulsetReconciler_Reconcile (0.01s)
    --- PASS: TestMyStatefulsetReconciler_Reconcile/Valid_MyStatefulset (0.00s)
    --- PASS: TestMyStatefulsetReconciler_Reconcile/Scale_Up_Scenario (0.00s)
    --- PASS: TestMyStatefulsetReconciler_Reconcile/Scale_Down_Scenario (0.00s)
    --- PASS: TestMyStatefulsetReconciler_Reconcile/Update_Pod_Template (0.00s)
=== RUN   TestMyStatefulsetReconciler_createPod
--- PASS: TestMyStatefulsetReconciler_createPod (0.00s)
=== RUN   TestMyStatefulsetReconciler_updateStatus
=== RUN   TestMyStatefulsetReconciler_updateStatus/All_Pods_Ready
--- PASS: TestMyStatefulsetReconciler_updateStatus (0.00s)
    --- PASS: TestMyStatefulsetReconciler_updateStatus/All_Pods_Ready (0.00s)
=== RUN   TestIsPodReady
=== RUN   TestIsPodReady/Pod_is_ready
=== RUN   TestIsPodReady/Pod_is_not_ready_-_wrong_phase
=== RUN   TestIsPodReady/Pod_is_not_ready_-_condition_false
--- PASS: TestIsPodReady (0.00s)
    --- PASS: TestIsPodReady/Pod_is_ready (0.00s)
    --- PASS: TestIsPodReady/Pod_is_not_ready_-_wrong_phase (0.00s)
    --- PASS: TestIsPodReady/Pod_is_not_ready_-_condition_false (0.00s)
=== RUN   TestAPIs
    suite_test.go:47: Skipping integration tests
--- SKIP: TestAPIs (0.00s)
PASS
coverage: 40.2% of statements
ok  	github.com/bryant-rh/my-statefulset/controllers	2.949s	coverage: 40.2% of statements

```
