# DevOps开发 golang 测试

1. 72小时内完成
2. fork本仓库
3. 通过kubebuilder或者手动创建golang的项目；完成自定义CRD MyStatefulSet核心功能的开发（功能同kubernetes StatefulSet），尽可能的完善！
4. 同时还需要实现ValidatingAdmissionWebhook，为MyStatefulSet提供准入验证
5. 要求不能直接引用kubernetes StatefulSet模块源码
6. 单元测试覆盖率必须大于80%，提交PR的时候，请附带上单元测试覆盖率截图或记录
7. 必须包含controller、AdmissionWebhook部署需要helm chart
8. makefile包含完整的编译流程（controller、AdmissionWebhook镜像的编译、helm chart编译、单元测试）
9. 完成以后通过pull request 提交，并备注面试姓名+联系方式，然后即时联系HR以免超时；

谢谢合作


# 单元测试
```Bash
# 运行单元测试
make test-unit

# 查看测试覆盖率
make test-coverage

# 运行特定测试
make test-specific TEST_PATTERN=TestMyStatefulsetReconciler

```
