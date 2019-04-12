package operator

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	zv1 "github.com/zalando-incubator/es-operator/pkg/apis/zalando.org/v1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

type mockResource struct {
	apiVersion    string
	kind          string
	name          string
	namespace     string
	uid           types.UID
	generation    int64
	labels        map[string]string
	labelSelector map[string]string
	replicas      int32
	eds           *zv1.ElasticsearchDataSet

	podTemplateSpec      *v1.PodTemplateSpec
	volumeClaimTemplates []v1.PersistentVolumeClaim
}

func (r *mockResource) Name() string                         { return r.name }
func (r *mockResource) Namespace() string                    { return r.namespace }
func (r *mockResource) APIVersion() string                   { return r.apiVersion }
func (r *mockResource) Kind() string                         { return r.kind }
func (r *mockResource) Labels() map[string]string            { return r.labels }
func (r *mockResource) LabelSelector() map[string]string     { return r.labelSelector }
func (r *mockResource) Generation() int64                    { return r.generation }
func (r *mockResource) UID() types.UID                       { return r.uid }
func (r *mockResource) Replicas() int32                      { return r.replicas }
func (r *mockResource) PodTemplateSpec() *v1.PodTemplateSpec { return r.podTemplateSpec }
func (r *mockResource) VolumeClaimTemplates() []v1.PersistentVolumeClaim {
	return r.volumeClaimTemplates
}
func (r *mockResource) Self() runtime.Object                           { return r.eds }
func (r *mockResource) EnsureResources() error                         { return nil }
func (r *mockResource) UpdateStatus(sts *appsv1.StatefulSet) error     { return nil }
func (r *mockResource) PreScaleDownHook(ctx context.Context) error     { return nil }
func (r *mockResource) OnStableReplicasHook(ctx context.Context) error { return nil }
func (r *mockResource) Drain(ctx context.Context, pod *v1.Pod) error   { return nil }

func TestPrioritizePodsForUpdate(t *testing.T) {
	updatingPod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:         "sts-4",
			GenerateName: "sts-",
			Annotations: map[string]string{
				operatorPodDrainingAnnotationKey: "",
			},
			Labels: map[string]string{
				controllerRevisionHashLabelKey: "",
			},
			Namespace: "default",
		},
		Spec: v1.PodSpec{
			NodeName: "node1",
		},
	}

	stsPod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:         "sts-1",
			GenerateName: "sts-",
			Annotations:  map[string]string{},
			Labels: map[string]string{
				controllerRevisionHashLabelKey: "hash",
			},
			Namespace: "default",
		},
		Spec: v1.PodSpec{
			NodeName: "node1",
		},
	}

	stsPod0 := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:         "sts-0",
			GenerateName: "sts-",
			Annotations:  map[string]string{},
			Labels: map[string]string{
				controllerRevisionHashLabelKey: "hash",
			},
			Namespace: "default",
		},
		Spec: v1.PodSpec{
			NodeName: "node1",
		},
	}

	podNoNode := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:         "sts-2",
			GenerateName: "sts-",
			Annotations:  map[string]string{},
			Labels: map[string]string{
				controllerRevisionHashLabelKey: "hash",
			},
			Namespace: "default",
		},
		Spec: v1.PodSpec{
			NodeName: "",
		},
	}

	podUpToDate := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:         "sts-3",
			GenerateName: "sts-",
			Annotations:  map[string]string{},
			Labels: map[string]string{
				controllerRevisionHashLabelKey: "hash",
			},
			Namespace: "default",
		},
		Spec: v1.PodSpec{
			NodeName: "node2",
		},
	}

	sr := &mockResource{
		eds: &zv1.ElasticsearchDataSet{
			TypeMeta:   metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{},
			Spec:       zv1.ElasticsearchDataSetSpec{},
			Status:     zv1.ElasticsearchDataSetStatus{},
		},
	}

	sts := &appsv1.StatefulSet{
		Status: appsv1.StatefulSetStatus{
			UpdateRevision: "hash",
		},
	}

	nodes := map[string]v1.Node{
		"node2": {},
	}

	pods := []v1.Pod{updatingPod}

	sortedPods, err := prioritizePodsForUpdate(pods, sts, sr, nodes)
	assert.NoError(t, err)
	assert.Len(t, sortedPods, 1)
	assert.Equal(t, updatingPod, sortedPods[0])

	// updating pod should be prioritized over stsPod
	pods = []v1.Pod{stsPod, updatingPod}
	sortedPods, err = prioritizePodsForUpdate(pods, sts, sr, nodes)
	assert.NoError(t, err)
	assert.Len(t, sortedPods, 2)
	assert.Equal(t, updatingPod, sortedPods[0])

	// stsPods should be sorted by ordinal number
	pods = []v1.Pod{stsPod, stsPod0}
	sortedPods, err = prioritizePodsForUpdate(pods, sts, sr, nodes)
	assert.NoError(t, err)
	assert.Len(t, sortedPods, 2)
	assert.Equal(t, stsPod0, sortedPods[0])

	// don't prioritize pods not on a node.
	pods = []v1.Pod{podNoNode}
	sortedPods, err = prioritizePodsForUpdate(pods, sts, sr, nodes)
	assert.NoError(t, err)
	assert.Len(t, sortedPods, 0)

	// don't prioritize pods part of unscaled statefulset
	desiredReplicas := int32(1)
	currentReplicas := int32(3)
	sr = &mockResource{
		replicas: desiredReplicas,
	}

	sts = &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Replicas: &currentReplicas,
		},
		Status: appsv1.StatefulSetStatus{
			UpdateRevision: "hash",
		},
	}

	pods = []v1.Pod{podUpToDate}
	sortedPods, err = prioritizePodsForUpdate(pods, sts, sr, nodes)
	assert.NoError(t, err)
	assert.Len(t, sortedPods, 0)
}

func TestSortStatefulSetPods(t *testing.T) {
	pods := make([]v1.Pod, 0, 13)
	for _, num := range []int{12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0} {
		pods = append(pods, v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:         fmt.Sprintf("sts-%d", num),
				GenerateName: "sts-",
			},
		})
	}

	sortedPods, err := sortStatefulSetPods(pods)
	assert.NoError(t, err)
	assert.Len(t, sortedPods, 13)
	assert.Equal(t, sortedPods[12].Name, "sts-12")
	assert.Equal(t, sortedPods[0].Name, "sts-0")
}
