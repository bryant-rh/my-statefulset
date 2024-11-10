package controllers

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"time"

	appsv1 "github.com/bryant-rh/my-statefulset/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	myStatefulsetFinalizer = "mystatefulset.bryant-rh/finalizer"
	controllerName         = "mystatefulset-controller"
)

//+kubebuilder:rbac:groups=apps.mystatefulset.com,resources=mystatefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.mystatefulset.com,resources=mystatefulsets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.mystatefulset.com,resources=mystatefulsets/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods/status,verbs=get
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

//+kubebuilder:printcolumn:name="Desired",type="integer",JSONPath=".spec.replicas",description="Desired number of pods"
//+kubebuilder:printcolumn:name="Current",type="integer",JSONPath=".status.replicas",description="Current number of pods"
//+kubebuilder:printcolumn:name="Ready",type="integer",JSONPath=".status.readyReplicas",description="Number of pods ready"
//+kubebuilder:printcolumn:name="Updated",type="integer",JSONPath=".status.updatedReplicas",description="Number of pods updated"
//+kubebuilder:printcolumn:name="Available",type="integer",JSONPath=".status.availableReplicas",description="Number of pods available"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
//+kubebuilder:subresource:status
//+kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas,selectorpath=.spec.selector

// MyStatefulsetReconciler reconciles a MyStatefulset object
type MyStatefulsetReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	Recorder    record.EventRecorder
	PodInformer cache.SharedIndexInformer
	PVCInformer cache.SharedIndexInformer
}

// Reconcile is part of the main kubernetes reconciliation loop
func (r *MyStatefulsetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Starting reconciliation", "request", req)

	// 获取 MyStatefulset 实例
	var mystatefulset appsv1.MyStatefulset
	if err := r.Get(ctx, req.NamespacedName, &mystatefulset); err != nil {
		if errors.IsNotFound(err) {
			log.Info("MyStatefulset not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get MyStatefulset")
		return ctrl.Result{}, err
	}

	// 处理删除
	if !mystatefulset.DeletionTimestamp.IsZero() {
		log.Info("MyStatefulset is being deleted",
			"name", mystatefulset.Name,
			"deletionTimestamp", mystatefulset.DeletionTimestamp)
		return r.handleDeletion(ctx, &mystatefulset)
	}

	// 打印完整的对象结构
	log.Info("Full MyStatefulset object",
		"spec", mystatefulset.Spec,
		"template", mystatefulset.Spec.Template,
		"template.metadata", mystatefulset.Spec.Template.ObjectMeta,
		"template.labels", mystatefulset.Spec.Template.ObjectMeta.Labels,
		"selector", mystatefulset.Spec.Selector)

	// Add selector validation after getting mystatefulset
	if mystatefulset.Spec.Selector == nil {
		log.Error(nil, "Selector is nil")
		r.Recorder.Event(&mystatefulset, corev1.EventTypeWarning, "InvalidSpec", "Selector cannot be nil")
		return ctrl.Result{}, fmt.Errorf("selector cannot be nil")
	}

	log.Info("Reconciling MyStatefulset",
		"name", mystatefulset.Name,
		"namespace", mystatefulset.Namespace,
		"replicas", mystatefulset.Spec.Replicas,
		"selector", mystatefulset.Spec.Selector.MatchLabels,
		"template_labels", mystatefulset.Spec.Template.Labels)

	// Add validation for template labels
	if mystatefulset.Spec.Template.ObjectMeta.Labels == nil {
		err := fmt.Errorf("pod template labels are required")
		r.Recorder.Event(&mystatefulset, corev1.EventTypeWarning, "ValidationFailed", err.Error())
		return ctrl.Result{}, err
	}

	// Validate that template labels match selector
	for key, value := range mystatefulset.Spec.Selector.MatchLabels {
		if v, ok := mystatefulset.Spec.Template.Labels[key]; !ok || v != value {
			err := fmt.Errorf("pod template labels must match selector")
			r.Recorder.Event(&mystatefulset, corev1.EventTypeWarning, "ValidationFailed", err.Error())
			return ctrl.Result{}, err
		}
	}

	// 添加 Finalizer
	if !controllerutil.ContainsFinalizer(&mystatefulset, myStatefulsetFinalizer) {
		controllerutil.AddFinalizer(&mystatefulset, myStatefulsetFinalizer)
		if err := r.Update(ctx, &mystatefulset); err != nil {
			return ctrl.Result{}, err
		}
	}

	// 验证 MyStatefulset
	if err := mystatefulset.Validate(); err != nil {
		r.Recorder.Event(&mystatefulset, corev1.EventTypeWarning, "ValidationFailed", err.Error())
		return ctrl.Result{}, err
	}

	// 验证 Service 存在（而不是创建）
	if mystatefulset.Spec.ServiceName != "" {
		service := &corev1.Service{}
		err := r.Get(ctx, types.NamespacedName{
			Name:      mystatefulset.Spec.ServiceName,
			Namespace: mystatefulset.Namespace,
		}, service)
		if err != nil {
			if errors.IsNotFound(err) {
				// Service 不存在，记录事件并返回错误
				r.Recorder.Event(&mystatefulset, corev1.EventTypeWarning, "ServiceNotFound",
					fmt.Sprintf("Required headless service %s not found", mystatefulset.Spec.ServiceName))
				return ctrl.Result{}, fmt.Errorf("headless service %s not found", mystatefulset.Spec.ServiceName)
			}
			return ctrl.Result{}, err
		}
	}

	// 确保 PVC 存在
	if err := r.reconcilePVCs(ctx, &mystatefulset); err != nil {
		return ctrl.Result{}, err
	}

	// 处理 Pod
	if err := r.reconcilePods(ctx, &mystatefulset); err != nil {
		log.Error(err, "Failed to reconcile pods",
			"mystatefulset", mystatefulset.Name,
			"namespace", mystatefulset.Namespace)
		return ctrl.Result{}, err
	}

	// 更新状态
	if err := r.updateStatus(ctx, &mystatefulset); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: time.Second * 30}, nil
}

// reconcilePVCs 确保 PVC 存在
func (r *MyStatefulsetReconciler) reconcilePVCs(ctx context.Context, mystatefulset *appsv1.MyStatefulset) error {
	for _, pvcTemplate := range mystatefulset.Spec.VolumeClaimTemplates {
		volumeName := "www" // 使用固定的名称
		if pvcTemplate.Name != "" {
			volumeName = pvcTemplate.Name
		}

		for ordinal := 0; ordinal < int(mystatefulset.Spec.Replicas); ordinal++ {
			pvcName := fmt.Sprintf("%s-%s-%d", volumeName, mystatefulset.Name, ordinal)

			pvc := &corev1.PersistentVolumeClaim{}
			err := r.Get(ctx, types.NamespacedName{
				Name:      pvcName,
				Namespace: mystatefulset.Namespace,
			}, pvc)

			if errors.IsNotFound(err) {
				// 创建新的 PVC
				newPVC := &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      pvcName,
						Namespace: mystatefulset.Namespace,
						Labels:    pvcTemplate.Labels,
						OwnerReferences: []metav1.OwnerReference{
							*metav1.NewControllerRef(mystatefulset, appsv1.GroupVersion.WithKind("MyStatefulset")),
						},
					},
					Spec: pvcTemplate.Spec,
				}

				if err := r.Create(ctx, newPVC); err != nil {
					return fmt.Errorf("failed to create PVC %s: %w", pvcName, err)
				}
			} else if err != nil {
				return err
			}
		}
	}
	return nil
}

// reconcilePods 处理 Pod 的创建、更新和删除
func (r *MyStatefulsetReconciler) reconcilePods(ctx context.Context, mystatefulset *appsv1.MyStatefulset) error {
	log := log.FromContext(ctx)

	// Add validation and logging for pod creation prerequisites
	if mystatefulset.Spec.Replicas == 0 {
		log.Info("Replicas is set to 0, no pods will be created")
		return nil
	}

	if len(mystatefulset.Spec.Template.Labels) == 0 {
		log.Error(nil, "Pod template labels are empty")
		return fmt.Errorf("pod template labels cannot be empty")
	}

	// Log selector and template labels match
	if !reflect.DeepEqual(mystatefulset.Spec.Selector.MatchLabels, mystatefulset.Spec.Template.Labels) {
		log.Error(nil, "Pod template labels don't match selector",
			"selector", mystatefulset.Spec.Selector.MatchLabels,
			"template_labels", mystatefulset.Spec.Template.Labels)
		return fmt.Errorf("pod template labels must match selector")
	}

	// Get existing pods with detailed logging
	existingPods := &corev1.PodList{}
	if err := r.List(ctx, existingPods,
		client.InNamespace(mystatefulset.Namespace),
		client.MatchingLabels(mystatefulset.Spec.Selector.MatchLabels)); err != nil {
		log.Error(err, "Failed to list pods",
			"namespace", mystatefulset.Namespace,
			"selector", mystatefulset.Spec.Selector.MatchLabels)
		return err
	}

	log.Info("Current pod status",
		"desired_replicas", mystatefulset.Spec.Replicas,
		"existing_pods", len(existingPods.Items),
		"selector", mystatefulset.Spec.Selector.MatchLabels)

	// 根据更新策略选择处理方式
	if mystatefulset.Spec.UpdateStrategy.Type == appsv1.RollingUpdateStatefulSetStrategyType {
		// 处理滚动更新
		partition := int32(0)
		if mystatefulset.Spec.UpdateStrategy.RollingUpdate != nil &&
			mystatefulset.Spec.UpdateStrategy.RollingUpdate.Partition != nil {
			partition = *mystatefulset.Spec.UpdateStrategy.RollingUpdate.Partition
		}

		// 按序号排序 pods（降序，从高到低）
		sort.Slice(existingPods.Items, func(i, j int) bool {
			return getOrdinal(existingPods.Items[i].Name) > getOrdinal(existingPods.Items[j].Name)
		})

		// 处理现有 Pod 的更新
		for _, pod := range existingPods.Items {
			ordinal := getOrdinal(pod.Name)
			if ordinal >= int(partition) {
				if needsUpdate(&pod, mystatefulset) {
					if err := r.Delete(ctx, &pod); err != nil && !errors.IsNotFound(err) {
						return err
					}
					// 等待 Pod 被删除
					if err := r.waitForPodDeletion(ctx, pod.Name, pod.Namespace); err != nil {
						return err
					}
					// 创建新的 Pod
					if err := r.createPod(ctx, mystatefulset, ordinal); err != nil {
						return err
					}
					// 每次只更新一个 Pod
					return nil
				}
			}
		}
	}

	// 处理常规的 Pod 创建和删除
	for i := 0; i < int(mystatefulset.Spec.Replicas); i++ {
		podName := fmt.Sprintf("%s-%d", mystatefulset.Name, i)
		log.Info("Checking pod", "podName", podName)

		var existingPod corev1.Pod
		err := r.Get(ctx, types.NamespacedName{
			Namespace: mystatefulset.Namespace,
			Name:      podName,
		}, &existingPod)

		if err != nil {
			if !errors.IsNotFound(err) {
				log.Error(err, "Error getting pod", "podName", podName)
				return err
			}

			log.Info("Pod does not exist, will create",
				"podName", podName,
				"namespace", mystatefulset.Namespace)

			// Create new pod with additional logging
			if err := r.createPod(ctx, mystatefulset, i); err != nil {
				log.Error(err, "Failed to create pod",
					"podName", podName,
					"error", err)
				return err
			}
		}
	}

	// 删除多余的 Pods
	for _, pod := range existingPods.Items {
		ordinal := getOrdinal(pod.Name)
		if ordinal >= int(mystatefulset.Spec.Replicas) {
			if err := r.Delete(ctx, &pod); err != nil && !errors.IsNotFound(err) {
				return err
			}
		}
	}

	log.V(1).Info("Reconciling pods",
		"existingPods", len(existingPods.Items),
		"desiredReplicas", mystatefulset.Spec.Replicas,
	)

	return nil
}

// createPod 创建新的 Pod
func (r *MyStatefulsetReconciler) createPod(ctx context.Context, mystatefulset *appsv1.MyStatefulset, ordinal int) error {
	log := log.FromContext(ctx)
	podName := fmt.Sprintf("%s-%d", mystatefulset.Name, ordinal)

	// Add pre-creation validation
	if mystatefulset.Spec.Template.Spec.Containers == nil || len(mystatefulset.Spec.Template.Spec.Containers) == 0 {
		return fmt.Errorf("pod template must contain at least one container")
	}

	// Create pod with additional logging
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: mystatefulset.Namespace,
			Labels:    mystatefulset.Spec.Template.Labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(mystatefulset, appsv1.GroupVersion.WithKind("MyStatefulset")),
			},
		},
		Spec: *mystatefulset.Spec.Template.Spec.DeepCopy(),
	}

	// 设置 hostname 和 subdomain
	pod.Spec.Hostname = podName
	if mystatefulset.Spec.ServiceName != "" {
		pod.Spec.Subdomain = mystatefulset.Spec.ServiceName
	}

	// 初始化 volumes 数组
	pod.Spec.Volumes = []corev1.Volume{}

	// 为每个 PVC 模板创建 volume
	for _, pvcTemplate := range mystatefulset.Spec.VolumeClaimTemplates {
		volumeName := "www" // 使用固定的名称，与 volumeMount 匹配
		if pvcTemplate.Name != "" {
			volumeName = pvcTemplate.Name
		}

		pvcName := fmt.Sprintf("%s-%s-%d", volumeName, mystatefulset.Name, ordinal)

		volume := corev1.Volume{
			Name: volumeName, // 使用相同的名称
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		}

		log.Info("Adding volume to pod",
			"volumeName", volume.Name,
			"pvcName", pvcName)

		pod.Spec.Volumes = append(pod.Spec.Volumes, volume)
	}

	log.Info("Creating pod with volumes",
		"pod", pod.Name,
		"volumeCount", len(pod.Spec.Volumes),
		"volumes", pod.Spec.Volumes,
		"volumeMounts", pod.Spec.Containers[0].VolumeMounts)

	err := r.Create(ctx, pod)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			log.Info("Pod already exists", "pod", podName)
			return err // 返回错误，但会在上层被处理
		}
		return fmt.Errorf("failed to create Pod %s: %w", podName, err)
	}

	log.Info("Successfully created pod", "pod", podName)
	return nil
}

// updateStatus 更新 MyStatefulset 状态
func (r *MyStatefulsetReconciler) updateStatus(ctx context.Context, mystatefulset *appsv1.MyStatefulset) error {
	log := log.FromContext(ctx)

	podList := &corev1.PodList{}
	if err := r.List(ctx, podList, client.InNamespace(mystatefulset.Namespace),
		client.MatchingLabels(mystatefulset.Spec.Selector.MatchLabels)); err != nil {
		log.Error(err, "Failed to list pods")
		return err
	}

	log.Info("Found pods for MyStatefulset",
		"podCount", len(podList.Items),
		"selector", mystatefulset.Spec.Selector.MatchLabels)

	var readyReplicas, currentReplicas, updatedReplicas, availableReplicas int32

	// 遍历所有 Pod 并更新计数
	for i, pod := range podList.Items {
		currentReplicas++

		log.Info("Processing pod",
			"podName", pod.Name,
			"index", i,
			"phase", pod.Status.Phase,
			"labels", pod.Labels)

		if isPodReady(&pod) {
			readyReplicas++
			log.Info("Pod is ready", "podName", pod.Name)
		}

		if isPodUpdated(&pod, mystatefulset) {
			updatedReplicas++
			log.Info("Pod is updated", "podName", pod.Name)
		}

		if isPodAvailable(&pod, mystatefulset.Spec.MinReadySeconds) {
			availableReplicas++
			log.Info("Pod is available", "podName", pod.Name)
		}
	}

	// 记录旧状态
	oldStatus := mystatefulset.Status.DeepCopy()

	// 更新状态
	newStatus := appsv1.MyStatefulsetStatus{
		ObservedGeneration: mystatefulset.Generation,
		Replicas:           currentReplicas,
		ReadyReplicas:      readyReplicas,
		CurrentReplicas:    currentReplicas,
		UpdatedReplicas:    updatedReplicas,
		AvailableReplicas:  availableReplicas,
	}

	log.Info("Status update",
		"oldStatus", oldStatus,
		"newStatus", newStatus,
		"currentReplicas", currentReplicas,
		"readyReplicas", readyReplicas,
		"updatedReplicas", updatedReplicas,
		"availableReplicas", availableReplicas)

	// 只有在状态发生变化时才更新
	if !reflect.DeepEqual(oldStatus, newStatus) {
		mystatefulset.Status = newStatus
		if err := r.Status().Update(ctx, mystatefulset); err != nil {
			log.Error(err, "Failed to update MyStatefulset status")
			return err
		}
		log.Info("Successfully updated status")
	} else {
		log.Info("Status unchanged, skipping update")
	}

	return nil
}

// handleDeletion 处理删除操作
func (r *MyStatefulsetReconciler) handleDeletion(ctx context.Context, mystatefulset *appsv1.MyStatefulset) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Handling deletion", "name", mystatefulset.Name, "namespace", mystatefulset.Namespace)

	// 创建一个带超时的上下文
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 检查是否仍然存在 Pod
	podList := &corev1.PodList{}
	if err := r.List(timeoutCtx, podList,
		client.InNamespace(mystatefulset.Namespace),
		client.MatchingLabels(mystatefulset.Spec.Selector.MatchLabels)); err != nil {
		log.Error(err, "Failed to list pods")
		return ctrl.Result{}, err
	}

	// 如果还有 Pod 存在，按照逆序删除
	if len(podList.Items) > 0 {
		// 按序号排序（降序）
		sort.Slice(podList.Items, func(i, j int) bool {
			return getOrdinal(podList.Items[i].Name) > getOrdinal(podList.Items[j].Name)
		})

		// 删除最后一个 Pod
		pod := &podList.Items[0]
		log.Info("Deleting pod", "pod", pod.Name)

		// 检查 Pod 是否已经被标记为删除
		if pod.DeletionTimestamp != nil {
			log.Info("Pod is already being deleted", "pod", pod.Name)
			// 如果 Pod 正在删除中，等待短暂时间后重新排队
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}

		// 删除 Pod
		if err := r.Delete(timeoutCtx, pod); err != nil {
			if !errors.IsNotFound(err) {
				log.Error(err, "Failed to delete pod", "pod", pod.Name)
				return ctrl.Result{}, err
			}
			// Pod 已经不存在，继续处理
			log.Info("Pod already deleted", "pod", pod.Name)
		}

		// 重新排队以检查剩余的 Pod
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// 检查 PVC
	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := r.List(timeoutCtx, pvcList,
		client.InNamespace(mystatefulset.Namespace),
		client.MatchingLabels(mystatefulset.Spec.Selector.MatchLabels)); err != nil {
		log.Error(err, "Failed to list PVCs")
		return ctrl.Result{}, err
	}

	// 如果还有 PVC 存在，删除它们
	if len(pvcList.Items) > 0 {
		pvc := &pvcList.Items[0]
		log.Info("Deleting PVC", "pvc", pvc.Name)

		if err := r.Delete(timeoutCtx, pvc); err != nil {
			if !errors.IsNotFound(err) {
				log.Error(err, "Failed to delete PVC", "pvc", pvc.Name)
				return ctrl.Result{}, err
			}
			log.Info("PVC already deleted", "pvc", pvc.Name)
		}

		// 重新排队以检查剩余的 PVC
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// 所有资源都已清理，移除 finalizer
	if controllerutil.ContainsFinalizer(mystatefulset, myStatefulsetFinalizer) {
		log.Info("Removing finalizer")
		controllerutil.RemoveFinalizer(mystatefulset, myStatefulsetFinalizer)
		if err := r.Update(timeoutCtx, mystatefulset); err != nil {
			if !errors.IsNotFound(err) {
				log.Error(err, "Failed to remove finalizer")
				return ctrl.Result{}, err
			}
			// 资源已经被删除，直接返回
			return ctrl.Result{}, nil
		}
	}

	log.Info("Successfully handled deletion")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MyStatefulsetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor(controllerName)

	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.MyStatefulset{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}

// 工具函数
func getOrdinal(podName string) int {
	ordinalStr := podName[len(podName)-1:]
	ordinal, _ := strconv.Atoi(ordinalStr)
	return ordinal
}

func needsUpdate(pod *corev1.Pod, mystatefulset *appsv1.MyStatefulset) bool {
	// 检查 Pod 标签
	if !reflect.DeepEqual(pod.Labels, mystatefulset.Spec.Template.Labels) {
		return true
	}

	// 检查容器规格
	if len(pod.Spec.Containers) != len(mystatefulset.Spec.Template.Spec.Containers) {
		return true
	}

	// 检查每个容器的镜像和资源
	for i, container := range pod.Spec.Containers {
		templateContainer := mystatefulset.Spec.Template.Spec.Containers[i]
		if container.Image != templateContainer.Image {
			return true
		}
		if !reflect.DeepEqual(container.Resources, templateContainer.Resources) {
			return true
		}
	}

	return false
}

func isPodReady(pod *corev1.Pod) bool {
	// Pod 必须处于 Running 阶段
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}

	// 检查 Ready 条件
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

// func isPodCurrent(pod *corev1.Pod, mystatefulset *appsv1.MyStatefulset) bool {
// 	// 这里可以添加更多的检查逻辑
// 	return true
// }

func isPodUpdated(pod *corev1.Pod, mystatefulset *appsv1.MyStatefulset) bool {
	// Pod 必须处于 Running 阶段
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}

	// 检查容器镜像
	if len(pod.Spec.Containers) != len(mystatefulset.Spec.Template.Spec.Containers) {
		return false
	}

	for i, container := range pod.Spec.Containers {
		if container.Image != mystatefulset.Spec.Template.Spec.Containers[i].Image {
			return false
		}
	}

	// 检查标签
	for key, value := range mystatefulset.Spec.Template.Labels {
		if pod.Labels[key] != value {
			return false
		}
	}

	return true
}

func isPodAvailable(pod *corev1.Pod, minReadySeconds int32) bool {
	if !isPodReady(pod) {
		return false
	}

	// 如果没有设置 minReadySeconds，则认为 Ready 就是 Available
	if minReadySeconds == 0 {
		return true
	}

	// 检查 Pod 是否已经就绪足够长的时间
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			readyTime := condition.LastTransitionTime.Time
			return time.Since(readyTime) >= time.Duration(minReadySeconds)*time.Second
		}
	}

	return false
}

// 添加新的辅助函数来等待 Pod 删除
func (r *MyStatefulsetReconciler) waitForPodDeletion(ctx context.Context, name, namespace string) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return wait.PollImmediateUntil(time.Second, func() (bool, error) {
		var pod corev1.Pod
		err := r.Get(timeoutCtx, types.NamespacedName{
			Namespace: namespace,
			Name:      name,
		}, &pod)

		if errors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		return false, nil
	}, timeoutCtx.Done())
}

// 添加自定义错误类型
type ReconcileError struct {
	Message string
	Requeue bool
}

func (e *ReconcileError) Error() string {
	return e.Message
}
