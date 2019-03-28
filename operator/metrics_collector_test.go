package operator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

func TestCalculateMedian(t *testing.T) {
	assert.Equal(t, int32(4), calculateMedian([]int32{1, 4, 6}))
	assert.Equal(t, int32(1), calculateMedian([]int32{1, 4}))
}

func TestGetCPUUsagePercent(t *testing.T) {
	pods := []v1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pod-x",
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name: "a",
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceCPU: resource.MustParse("100m"),
							},
						},
					},
					{
						Name: "b",
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceCPU: resource.MustParse("1000m"),
							},
						},
					},
				},
			},
		},
	}

	podMetrics := []v1beta1.PodMetrics{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pod-x",
			},
			Containers: []v1beta1.ContainerMetrics{
				{
					Name: "a",
					Usage: v1.ResourceList{
						v1.ResourceCPU: resource.MustParse("50m"),
					},
				},
				{
					Name: "b",
					Usage: v1.ResourceList{
						v1.ResourceCPU: resource.MustParse("0m"),
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pod-y",
			},
			Containers: []v1beta1.ContainerMetrics{
				{
					Name: "a",
					Usage: v1.ResourceList{
						v1.ResourceCPU: resource.MustParse("50m"),
					},
				},
				{
					Name: "b",
					Usage: v1.ResourceList{
						v1.ResourceCPU: resource.MustParse("0m"),
					},
				},
			},
		},
	}

	cpuUsagePercent := getCPUUsagePercent(podMetrics, pods)
	require.Len(t, cpuUsagePercent, 1)
	require.EqualValues(t, 50, cpuUsagePercent[0])
}
