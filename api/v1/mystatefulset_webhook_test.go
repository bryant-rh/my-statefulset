package v1

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMyStatefulset_ValidateCreate(t *testing.T) {
	tests := []struct {
		name    string
		ms      *MyStatefulset
		wantErr bool
	}{
		{
			name: "valid replicas",
			ms: &MyStatefulset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mystatefulset",
					Namespace: "default",
				},
				Spec: MyStatefulsetSpec{
					Replicas:    3,
					ServiceName: "test-service",
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test",
						},
					},
					Template: PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "nginx",
									Image: "nginx:latest",
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid replicas",
			ms: &MyStatefulset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mystatefulset-invalid",
					Namespace: "default",
				},
				Spec: MyStatefulsetSpec{
					Replicas:    -1,
					ServiceName: "test-service",
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test",
						},
					},
					Template: PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "nginx",
									Image: "nginx:latest",
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ms.ValidateCreate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCreate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMyStatefulset_ValidateUpdate(t *testing.T) {
	oldMs := &MyStatefulset{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-mystatefulset",
			Namespace: "default",
		},
		Spec: MyStatefulsetSpec{
			Replicas:    1,
			ServiceName: "test-service",
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "test",
				},
			},
			Template: PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "test",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:latest",
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name    string
		ms      *MyStatefulset
		wantErr bool
	}{
		{
			name: "valid update",
			ms: &MyStatefulset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mystatefulset",
					Namespace: "default",
				},
				Spec: MyStatefulsetSpec{
					Replicas:    2,
					ServiceName: "test-service",
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test",
						},
					},
					Template: PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "nginx",
									Image: "nginx:latest",
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ms.ValidateUpdate(oldMs)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUpdate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
