package clientset

import (
	"fmt"

	clientset "github.com/zalando-incubator/es-operator/pkg/client/clientset/versioned"
	zalandov1 "github.com/zalando-incubator/es-operator/pkg/client/clientset/versioned/typed/zalando.org/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
	"k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
)

type ZInterface = clientset.Interface

type Clientset struct {
	kubernetes.Interface
	zInterface clientset.Interface
	mInterface metrics.Interface
}

func (c *Clientset) ZalandoV1() zalandov1.ZalandoV1Interface {
	return c.zInterface.ZalandoV1()
}

func (c *Clientset) MetricsV1Beta1() v1beta1.MetricsV1beta1Interface {
	return c.mInterface.MetricsV1beta1()
}

func NewClientset(kubeConfig *rest.Config) (*Clientset, error) {
	client, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to setup Kubernetes client: %v", err)
	}

	zClient, err := clientset.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to setup Kubernetes CRD client: %v", err)
	}

	mClient, err := metrics.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to setup Kubernetes metrics client: %v", err)
	}

	return &Clientset{
		Interface:  client,
		zInterface: zClient,
		mInterface: mClient,
	}, nil
}
