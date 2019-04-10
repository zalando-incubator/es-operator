package main

import (
	"fmt"
	"net/url"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/zalando-incubator/es-operator/operator"
	clientset "github.com/zalando-incubator/es-operator/pkg/client/clientset/versioned"
	zv1client "github.com/zalando-incubator/es-operator/pkg/client/clientset/versioned/typed/zalando.org/v1"

	"k8s.io/client-go/kubernetes"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	kubernetesClient, edsClient = createClients()
	namespace                   = requiredEnvar("E2E_NAMESPACE")
	operatorId                  = requiredEnvar("OPERATOR_ID")
)

func init() {
	logrus.SetFormatter(&logrus.TextFormatter{ForceColors: true})
}

func createClients() (kubernetes.Interface, clientset.Interface) {
	kubeconfig := os.Getenv("KUBECONFIG")

	var cfg *rest.Config
	var err error
	if kubeconfig != "" {
		cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		cfg, err = rest.InClusterConfig()
	}
	if err != nil {
		panic(err)
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		panic(err)
	}
	edsClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		panic(err)
	}
	return kubeClient, edsClient
}

func edsInterface() zv1client.ElasticsearchDataSetInterface {
	return edsClient.ZalandoV1().ElasticsearchDataSets(namespace)
}

func statefulSetInterface() appsv1.StatefulSetInterface {
	return kubernetesClient.AppsV1().StatefulSets(namespace)
}

func serviceInterface() v1.ServiceInterface {
	return kubernetesClient.CoreV1().Services(namespace)
}

func requiredEnvar(envar string) string {
	namespace := os.Getenv(envar)
	if namespace == "" {
		panic(fmt.Sprintf("%s not set", envar))
	}
	return namespace
}

func setupESClient(defaultServiceEndpoint string) (*operator.ESClient, error) {
	serviceEndpoint := os.Getenv("ES_SERVICE_ENDPOINT")
	if serviceEndpoint == "" {
		serviceEndpoint = defaultServiceEndpoint
	}
	endpoint, err := url.Parse(serviceEndpoint)
	if err != nil {
		return nil, err
	}
	return &operator.ESClient{Endpoint: endpoint}, nil
}
