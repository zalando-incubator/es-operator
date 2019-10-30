package main

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/zalando-incubator/es-operator/operator"
	"github.com/zalando-incubator/es-operator/pkg/clientset"
	"gopkg.in/alecthomas/kingpin.v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
)

const (
	defaultInterval           = "10s"
	defaultAutoscalerInterval = "30s"
	defaultMetricsAddress     = ":7979"
	defaultClientGoTimeout    = 30 * time.Second
	defaultClusterDNSZone     = "cluster.local."
)

var (
	config struct {
		Interval              time.Duration
		AutoscalerInterval    time.Duration
		APIServer             *url.URL
		PodSelectors          Labels
		PriorityNodeSelectors Labels
		MetricsAddress        string
		ClientGoTimeout       time.Duration
		Debug                 bool
		OperatorID            string
		Namespace             string
		ClusterDNSZone        string
		ElasticsearchEndpoint *url.URL
	}
)

func main() {
	config.PodSelectors = Labels(map[string]string{})
	config.PriorityNodeSelectors = Labels(map[string]string{})

	kingpin.Flag("debug", "Enable debug logging.").BoolVar(&config.Debug)
	kingpin.Flag("interval", "Interval between syncing.").
		Default(defaultInterval).DurationVar(&config.Interval)
	kingpin.Flag("autoscaler-interval", "Interval between checking if autoscaling is needed.").
		Default(defaultAutoscalerInterval).DurationVar(&config.AutoscalerInterval)
	kingpin.Flag("apiserver", "API server url.").URLVar(&config.APIServer)
	kingpin.Flag("pod-selector", "Operator will manage all pods selected by this label selector. <key>=<value>,+.").
		SetValue(&config.PodSelectors)
	kingpin.Flag("priority-node-selector", "Specify a label selector for finding nodes with the highest priority. Common use case for this is to priorize nodes that are ready over nodes that are about to be terminated.").
		SetValue(&config.PriorityNodeSelectors)
	kingpin.Flag("metrics-address", "defines where to serve metrics").
		Default(defaultMetricsAddress).StringVar(&config.MetricsAddress)
	kingpin.Flag("client-go-timeout", "Set the timeout used for the Kubernetes client").
		Default(defaultClientGoTimeout.String()).DurationVar(&config.ClientGoTimeout)
	kingpin.Flag("operator-id", "ID of the operator used to determine ownership of EDS resources").
		StringVar(&config.OperatorID)
	kingpin.Flag("cluster-dns-zone", "The zone used for the cluster internal DNS. Used when generating ES service endpoint").
		Default(defaultClusterDNSZone).StringVar(&config.ClusterDNSZone)
	kingpin.Flag("elasticsearch-endpoint", "The Elasticsearch endpoint to use for reaching Elasticsearch API. By default the service endpoint for the EDS is used").
		URLVar(&config.ElasticsearchEndpoint)
	kingpin.Flag("namespace", "Limit operator to a certain namespace").
		Default(v1.NamespaceAll).StringVar(&config.Namespace)

	kingpin.Parse()

	if config.Debug {
		log.SetLevel(log.DebugLevel)
	}

	ctx, cancel := context.WithCancel(context.Background())
	kubeConfig, err := configureKubeConfig(config.APIServer, defaultClientGoTimeout, ctx.Done())
	if err != nil {
		log.Fatalf("Failed to setup Kubernetes config: %v", err)
	}

	client, err := clientset.NewClientset(kubeConfig)
	if err != nil {
		log.Fatalf("Failed to setup Kubernetes client: %v", err)
	}

	operator := operator.NewElasticsearchOperator(
		client,
		config.PriorityNodeSelectors,
		config.Interval,
		config.AutoscalerInterval,
		config.OperatorID,
		config.Namespace,
		config.ClusterDNSZone,
		config.ElasticsearchEndpoint,
	)

	go handleSigterm(cancel)
	go serveMetrics(config.MetricsAddress)
	err = operator.Run(ctx)
	if err != nil {
		log.Fatalf("Failed to run operator: %v", err)
	}
}

// handleSigterm handles SIGTERM signal sent to the process.
func handleSigterm(cancelFunc func()) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM)
	<-signals
	log.Info("Received Term signal. Terminating...")
	cancelFunc()
}

// configureKubeConfig configures a kubeconfig.
func configureKubeConfig(apiServerURL *url.URL, timeout time.Duration, stopCh <-chan struct{}) (*rest.Config, error) {
	tr := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   timeout,
			KeepAlive: 30 * time.Second,
			DualStack: false, // K8s do not work well with IPv6
		}).DialContext,
		TLSHandshakeTimeout:   timeout,
		ResponseHeaderTimeout: 10 * time.Second,
		MaxIdleConns:          10,
		MaxIdleConnsPerHost:   2,
		IdleConnTimeout:       20 * time.Second,
	}

	// We need this to reliably fade on DNS change, which is right
	// now not fixed with IdleConnTimeout in the http.Transport.
	// https://github.com/golang/go/issues/23427
	go func(d time.Duration) {
		for {
			select {
			case <-time.After(d):
				tr.CloseIdleConnections()
			case <-stopCh:
				return
			}
		}
	}(20 * time.Second)

	if apiServerURL != nil {
		return &rest.Config{
			Host:      apiServerURL.String(),
			Timeout:   timeout,
			Transport: tr,
			QPS:       100.0,
			Burst:     500,
		}, nil
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	// patch TLS config
	restTransportConfig, err := config.TransportConfig()
	if err != nil {
		return nil, err
	}
	restTLSConfig, err := transport.TLSConfigFor(restTransportConfig)
	if err != nil {
		return nil, err
	}
	tr.TLSClientConfig = restTLSConfig

	config.Timeout = timeout
	config.Transport = tr
	config.QPS = 100.0
	config.Burst = 500
	// disable TLSClientConfig to make the custom Transport work
	config.TLSClientConfig = rest.TLSClientConfig{}
	return config, nil
}

// gather go metrics
func serveMetrics(address string) {
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(address, nil))
}
