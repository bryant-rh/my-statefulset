package controllers

import (
	"context"
	"fmt"
	"testing"

	appsv1 "github.com/bryant-rh/my-statefulset/api/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestMyStatefulsetReconciler_Reconcile(t *testing.T) {
	// 注册自定义资源
	s := runtime.NewScheme()
	_ = appsv1.AddToScheme(s)
	_ = corev1.AddToScheme(s)

	// 创建一个测试用的 EventRecorder
	recorder := record.NewFakeRecorder(100)

	tests := []struct {
		name          string
		myStatefulset *appsv1.MyStatefulset
		expectedError bool
		setupFunc     func(*testing.T, client.Client)
	}{
		{
			name: "Valid MyStatefulset",
			myStatefulset: &appsv1.MyStatefulset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-statefulset",
					Namespace: "default",
				},
				Spec: appsv1.MyStatefulsetSpec{
					Replicas:    3,
					ServiceName: "test-service",
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test",
						},
					},
					Template: appsv1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test-container",
									Image: "nginx:latest",
								},
							},
						},
					},
				},
			},
			expectedError: false,
			setupFunc: func(t *testing.T, c client.Client) {
				// 创建一些现有的 Pod
				for i := 0; i < 2; i++ {
					pod := createTestPod(fmt.Sprintf("test-statefulset-%d", i))
					require.NoError(t, c.Create(context.Background(), pod))
				}
			},
		},
		{
			name: "Scale Up Scenario",
			myStatefulset: &appsv1.MyStatefulset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-statefulset",
					Namespace: "default",
				},
				Spec: appsv1.MyStatefulsetSpec{
					Replicas:    3,
					ServiceName: "test-service",
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test",
						},
					},
					Template: appsv1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test-container",
									Image: "nginx:latest",
								},
							},
						},
					},
				},
			},
			expectedError: false,
			setupFunc: func(t *testing.T, c client.Client) {
				// 创建一些现有的 Pod
				for i := 0; i < 2; i++ {
					pod := createTestPod(fmt.Sprintf("test-statefulset-%d", i))
					require.NoError(t, c.Create(context.Background(), pod))
				}
			},
		},
		{
			name: "Scale Down Scenario",
			myStatefulset: &appsv1.MyStatefulset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-statefulset",
					Namespace: "default",
				},
				Spec: appsv1.MyStatefulsetSpec{
					Replicas:    1,
					ServiceName: "test-service",
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test",
						},
					},
					Template: appsv1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test-container",
									Image: "nginx:latest",
								},
							},
						},
					},
				},
			},
			expectedError: false,
			setupFunc: func(t *testing.T, c client.Client) {
				// 创建更多的 Pod
				for i := 0; i < 3; i++ {
					pod := createTestPod(fmt.Sprintf("test-statefulset-%d", i))
					require.NoError(t, c.Create(context.Background(), pod))
				}
			},
		},
		{
			name: "Update Pod Template",
			myStatefulset: &appsv1.MyStatefulset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-statefulset",
					Namespace: "default",
				},
				Spec: appsv1.MyStatefulsetSpec{
					Replicas:    3,
					ServiceName: "test-service",
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test",
						},
					},
					Template: appsv1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "updated-container",
									Image: "nginx:1.19",
								},
							},
						},
					},
				},
			},
			expectedError: false,
			setupFunc: func(t *testing.T, c client.Client) {
				// 创建旧版本的 Pod
				pod := createTestPod("test-statefulset-0")
				pod.Spec.Containers[0].Image = "nginx:1.18"
				require.NoError(t, c.Create(context.Background(), pod))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建 headless service
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					ClusterIP: "None", // 这使其成为 headless service
					Selector: map[string]string{
						"app": "test",
					},
					Ports: []corev1.ServicePort{
						{
							Port: 80,
						},
					},
				},
			}

			// 创建假客户端
			client := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(tt.myStatefulset, service).
				Build()

			// 如果有 setupFunc，执行它
			if tt.setupFunc != nil {
				tt.setupFunc(t, client)
			}

			// 创建 reconciler
			r := &MyStatefulsetReconciler{
				Client:      client,
				Scheme:      s,
				Recorder:    recorder,
				PodInformer: &fakePodInformer{},
				PVCInformer: &fakePVCInformer{},
			}

			// 执行 reconcile
			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      tt.myStatefulset.Name,
					Namespace: tt.myStatefulset.Namespace,
				},
			}

			_, err := r.Reconcile(context.Background(), req)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMyStatefulsetReconciler_createPod(t *testing.T) {
	s := runtime.NewScheme()
	_ = appsv1.AddToScheme(s)
	_ = corev1.AddToScheme(s)

	myStatefulset := &appsv1.MyStatefulset{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-statefulset",
			Namespace: "default",
		},
		Spec: appsv1.MyStatefulsetSpec{
			Replicas:    3,
			ServiceName: "test-service",
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "test",
				},
			},
			Template: appsv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "test",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "nginx:latest",
						},
					},
				},
			},
		},
	}

	client := fake.NewClientBuilder().WithScheme(s).Build()

	r := &MyStatefulsetReconciler{
		Client: client,
		Scheme: s,
	}

	err := r.createPod(context.Background(), myStatefulset, 0)
	assert.NoError(t, err)

	// 验证Pod是否被创建
	pod := &corev1.Pod{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      "test-statefulset-0",
		Namespace: "default",
	}, pod)
	assert.NoError(t, err)
	assert.Equal(t, "test-statefulset-0", pod.Name)
	assert.Equal(t, "default", pod.Namespace)
	assert.Equal(t, map[string]string{"app": "test"}, pod.Labels)
}

func TestMyStatefulsetReconciler_updateStatus(t *testing.T) {
	// 设置试环境
	s := runtime.NewScheme()
	_ = appsv1.AddToScheme(s)
	_ = corev1.AddToScheme(s)

	tests := []struct {
		name           string
		myStatefulset  *appsv1.MyStatefulset
		pods           []*corev1.Pod
		expectedStatus appsv1.MyStatefulsetStatus
	}{
		{
			name: "All Pods Ready",
			myStatefulset: &appsv1.MyStatefulset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-statefulset",
					Namespace: "default",
					UID:       "test-uid",
				},
				Spec: appsv1.MyStatefulsetSpec{
					Replicas: 3,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test",
						},
					},
				},
			},
			pods: []*corev1.Pod{
				createPodWithOwner("test-statefulset-0", "test-uid"),
				createPodWithOwner("test-statefulset-1", "test-uid"),
				createPodWithOwner("test-statefulset-2", "test-uid"),
			},
			expectedStatus: appsv1.MyStatefulsetStatus{
				Replicas:      3,
				ReadyReplicas: 3,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建 fake client
			client := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(tt.myStatefulset).
				Build()

			// 创建并存储所有的 Pod
			for _, pod := range tt.pods {
				err := client.Create(context.Background(), pod)
				require.NoError(t, err)
			}

			// 创建 reconciler
			r := &MyStatefulsetReconciler{
				Client: client,
				Scheme: s,
			}

			// 执行状态更新
			err := r.updateStatus(context.Background(), tt.myStatefulset)
			require.NoError(t, err)

			// 验证状态
			assert.Equal(t, tt.expectedStatus.Replicas, tt.myStatefulset.Status.Replicas)
			assert.Equal(t, tt.expectedStatus.ReadyReplicas, tt.myStatefulset.Status.ReadyReplicas)
		})
	}
}

func TestIsPodReady(t *testing.T) {
	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected bool
	}{
		{
			name: "Pod is ready",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "Pod is not ready - wrong phase",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "Pod is not ready - condition false",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionFalse,
						},
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPodReady(tt.pod)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// 辅助函数
func createTestPod(name string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Labels: map[string]string{
				"app": "test",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test-container",
					Image: "nginx:latest",
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
}

func createPodWithOwner(name, ownerUID string) *corev1.Pod {
	trueVal := true
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Labels: map[string]string{
				"app": "test",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: appsv1.GroupVersion.String(),
					Kind:       "MyStatefulset",
					Name:       "test-statefulset",
					UID:        types.UID(ownerUID),
					Controller: &trueVal,
				},
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test-container",
					Image: "nginx:latest",
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
}

type fakePVCInformer struct {
	cache.SharedIndexInformer
}

func (f *fakePVCInformer) GetStore() cache.Store {
	return &fakePVCStore{}
}

type fakePVCStore struct{}

func (s *fakePVCStore) Add(obj interface{}) error    { return nil }
func (s *fakePVCStore) Update(obj interface{}) error { return nil }
func (s *fakePVCStore) Delete(obj interface{}) error { return nil }
func (s *fakePVCStore) List() []interface{}          { return []interface{}{} }
func (s *fakePVCStore) ListKeys() []string           { return []string{} }
func (s *fakePVCStore) Get(obj interface{}) (item interface{}, exists bool, err error) {
	return nil, false, nil
}
func (s *fakePVCStore) GetByKey(key string) (item interface{}, exists bool, err error) {
	return nil, false, nil
}
func (s *fakePVCStore) Replace([]interface{}, string) error { return nil }
func (s *fakePVCStore) Resync() error                       { return nil }

// 在文件中添加 fakePodInformer 的实现
type fakePodInformer struct {
	cache.SharedIndexInformer
	pods []*corev1.Pod
}

func (f *fakePodInformer) GetStore() cache.Store {
	return &fakePodStore{pods: f.pods}
}

type fakePodStore struct {
	pods []*corev1.Pod
}

func (s *fakePodStore) Add(obj interface{}) error    { return nil }
func (s *fakePodStore) Update(obj interface{}) error { return nil }
func (s *fakePodStore) Delete(obj interface{}) error { return nil }
func (s *fakePodStore) List() []interface{} {
	result := make([]interface{}, len(s.pods))
	for i, pod := range s.pods {
		result[i] = pod
	}
	return result
}
func (s *fakePodStore) ListKeys() []string { return []string{} }
func (s *fakePodStore) Get(obj interface{}) (item interface{}, exists bool, err error) {
	return nil, false, nil
}
func (s *fakePodStore) GetByKey(key string) (item interface{}, exists bool, err error) {
	return nil, false, nil
}
func (s *fakePodStore) Replace([]interface{}, string) error { return nil }
func (s *fakePodStore) Resync() error                       { return nil }
