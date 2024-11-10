
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

# 单元测试
```Bash
# 运行单元测试
make test-unit

# 查看测试覆盖率
make test-coverage

# 运行特定测试
make test-specific TEST_PATTERN=TestMyStatefulsetReconciler

```
