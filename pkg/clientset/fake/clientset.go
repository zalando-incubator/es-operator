package fake

import (
	fakeclientset "github.com/zalando-incubator/es-operator/pkg/client/clientset/versioned/fake"
	zalandov1 "github.com/zalando-incubator/es-operator/pkg/client/clientset/versioned/typed/zalando.org/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/testing"
	fakemetrics "k8s.io/metrics/pkg/client/clientset/versioned/fake"
	"k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
)

type Clientset struct {
	testing.Fake
	kubernetes.Interface
	ZInterface fakeclientset.Clientset
	MInterface fakemetrics.Clientset
}

func (c *Clientset) ZalandoV1() zalandov1.ZalandoV1Interface {
	return c.ZInterface.ZalandoV1()
}

func (c *Clientset) MetricsV1Beta1() v1beta1.MetricsV1beta1Interface {
	return c.MInterface.MetricsV1beta1()
}

func NewSimpleClientset() *Clientset {
	return &Clientset{
		Interface:  fake.NewSimpleClientset(),
		ZInterface: *fakeclientset.NewSimpleClientset(),
		MInterface: *fakemetrics.NewSimpleClientset(),
	}
}
