package v1

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log 是 webhook 包的日志
var mystatefulsetlog = logf.Log.WithName("mystatefulset-resource")

// 添加常量定义
const (
	defaultReplicas = int32(1)
	minReplicas     = int32(0)
	maxReplicas     = int32(100) // 添加最大副本数限制
)

// SetupWebhookWithManager 将 webhook 注册到 manager 中
func (r *MyStatefulset) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-apps-my-domain-v1-mystatefulset,mutating=true,failurePolicy=fail,sideEffects=None,groups=apps.my.domain,resources=mystatefulsets,verbs=create;update,versions=v1,name=mmystatefulset.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &MyStatefulset{}

// Default 实现了 webhook.Defaulter 接口，用于设置默认值
func (r *MyStatefulset) Default() {
	mystatefulsetlog.Info("setting default values", "name", r.Name)

	// 设置默认副本数
	if r.Spec.Replicas == 0 {
		r.Spec.Replicas = defaultReplicas
	}

	// 设置默认标签
	if r.Labels == nil {
		r.Labels = make(map[string]string)
	}
	r.Labels["app"] = r.Name

	// 为每个容器设置默认资源限制
	for i := range r.Spec.Template.Spec.Containers {
		container := &r.Spec.Template.Spec.Containers[i]
		if container.Resources.Limits == nil {
			mystatefulsetlog.Info("setting default values for resource limits", "name", r.Name)

			// 设置默认资源限制
			// ... 根据需求设置
		}
	}
}

//+kubebuilder:webhook:path=/validate-apps-my-domain-v1-mystatefulset,mutating=false,failurePolicy=fail,sideEffects=None,groups=apps.my.domain,resources=mystatefulsets,verbs=create;update,versions=v1,name=vmystatefulset.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &MyStatefulset{}

// ValidateCreate 实现了 webhook.Validator 接口
func (r *MyStatefulset) ValidateCreate() error {
	mystatefulsetlog.Info("validating creation", "name", r.Name)
	return r.validateMyStatefulSet()
}

// ValidateUpdate 实现了 webhook.Validator 接口
func (r *MyStatefulset) ValidateUpdate(old runtime.Object) error {
	mystatefulsetlog.Info("validating update", "name", r.Name)

	// 转换旧对象
	oldMyStatefulset, ok := old.(*MyStatefulset)
	if !ok {
		return fmt.Errorf("expected a MyStatefulset but got a %T", old)
	}

	var allErrs field.ErrorList

	// 1. 验证基本字段
	if err := r.validateMyStatefulSet(); err != nil {
		return err
	}

	// 2. 验证更新特定的规则
	specPath := field.NewPath("spec")

	// 2.1 验证副本数的变化不能太大（可选的业务规则）
	if r.Spec.Replicas != oldMyStatefulset.Spec.Replicas {
		oldReplicas := oldMyStatefulset.Spec.Replicas
		newReplicas := r.Spec.Replicas
		if newReplicas > oldReplicas*2 {
			allErrs = append(allErrs, field.Invalid(
				specPath.Child("replicas"),
				r.Spec.Replicas,
				"cannot increase replicas by more than 100% in a single update"))
		}
	}

	// 2.2 验证不可变字段
	// 例如：不允许更改某些标签
	if oldMyStatefulset.Labels["app"] != r.Labels["app"] {
		allErrs = append(allErrs, field.Forbidden(
			field.NewPath("metadata").Child("labels").Child("app"),
			"app label is immutable"))
	}

	// 2.3 验证容器配置的更改
	for i, newContainer := range r.Spec.Template.Spec.Containers {
		// 找到对应的旧容器
		var oldContainer *corev1.Container
		for _, c := range oldMyStatefulset.Spec.Template.Spec.Containers {
			if c.Name == newContainer.Name {
				oldContainer = &c
				break
			}
		}

		if oldContainer != nil {
			containerPath := specPath.Child("template").Child("spec").Child("containers").Index(i)

			// 2.3.1 验证镜像更新策略
			if !isValidImageUpdate(oldContainer.Image, newContainer.Image) {
				allErrs = append(allErrs, field.Invalid(
					containerPath.Child("image"),
					newContainer.Image,
					"invalid image update"))
			}

			// 2.3.2 验证资源限制的更改
			if err := validateResourceUpdate(oldContainer, &newContainer, containerPath); err != nil {
				allErrs = append(allErrs, err)
			}
		}
	}

	// 2.4 验证存储配置的更改（如果适用）
	// ... 添加存储相关的验证 ...

	if len(allErrs) > 0 {
		return apierrors.NewInvalid(
			schema.GroupKind{Group: GroupVersion.Group, Kind: "MyStatefulset"},
			r.Name,
			allErrs)
	}

	return nil
}

// ValidateDelete 实现了 webhook.Validator 接口
func (r *MyStatefulset) ValidateDelete() error {
	mystatefulsetlog.Info("validating deletion", "name", r.Name)
	// 可以添加删除前的验证逻辑，比如检查依赖资源
	return nil
}

// validateMyStatefulSet 验证 MyStatefulSet 的通用逻辑
func (r *MyStatefulset) validateMyStatefulSet() error {
	var allErrs field.ErrorList

	// 验证副本数范围
	if r.Spec.Replicas < minReplicas {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec").Child("replicas"),
			r.Spec.Replicas,
			fmt.Sprintf("must be greater than or equal to %d", minReplicas)))
	}
	if r.Spec.Replicas > maxReplicas {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec").Child("replicas"),
			r.Spec.Replicas,
			fmt.Sprintf("must be less than or equal to %d", maxReplicas)))
	}

	// 验证容器配置
	if len(r.Spec.Template.Spec.Containers) == 0 {
		allErrs = append(allErrs, field.Required(
			field.NewPath("spec").Child("template").Child("spec").Child("containers"),
			"at least one container must be specified"))
	}

	// 验证每个容器的配置
	for i, container := range r.Spec.Template.Spec.Containers {
		containerPath := field.NewPath("spec").Child("template").Child("spec").Child("containers").Index(i)

		// 验证镜像
		if container.Image == "" {
			allErrs = append(allErrs, field.Required(
				containerPath.Child("image"),
				"container image is required"))
		}

		// 验证资源请求和限制
		if container.Resources.Limits != nil && container.Resources.Requests != nil {
			mystatefulsetlog.Info("setting default values for resource Requests and Limits", "name", r.Name)

			// 验证资源请求不超过限制
			// ... 添加资源验证逻辑
		}

		// 验证端口名称唯一性
		portNames := make(map[string]bool)
		for _, port := range container.Ports {
			if port.Name != "" {
				if portNames[port.Name] {
					allErrs = append(allErrs, field.Duplicate(
						containerPath.Child("ports").Key(port.Name),
						"port name must be unique"))
				}
				portNames[port.Name] = true
			}
		}
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: GroupVersion.Group, Kind: "MyStatefulset"},
		r.Name,
		allErrs)
}

// 辅助函数：验证镜像更新是否有效
func isValidImageUpdate(oldImage, newImage string) bool {
	// 示例：不允许从正式版本回退到测试版本
	if strings.Contains(oldImage, "prod") && strings.Contains(newImage, "test") {
		return false
	}
	return true
}

// 辅助函数：验证资源更新
func validateResourceUpdate(oldContainer, newContainer *corev1.Container, path *field.Path) *field.Error {
	// 示例：不允许减少资源限制
	if oldContainer.Resources.Limits != nil && newContainer.Resources.Limits != nil {
		oldCPU := oldContainer.Resources.Limits.Cpu()
		newCPU := newContainer.Resources.Limits.Cpu()
		if newCPU.Cmp(*oldCPU) < 0 {
			return field.Invalid(
				path.Child("resources").Child("limits").Child("cpu"),
				newContainer.Resources.Limits.Cpu(),
				"cannot decrease CPU limit")
		}

		oldMemory := oldContainer.Resources.Limits.Memory()
		newMemory := newContainer.Resources.Limits.Memory()
		if newMemory.Cmp(*oldMemory) < 0 {
			return field.Invalid(
				path.Child("resources").Child("limits").Child("memory"),
				newContainer.Resources.Limits.Memory(),
				"cannot decrease memory limit")
		}
	}
	return nil
}
