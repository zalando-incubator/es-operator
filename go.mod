module github.com/zalando-incubator/es-operator

require (
	github.com/cenk/backoff v2.2.1+incompatible
	github.com/gophercloud/gophercloud v0.1.0 // indirect
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/jarcoal/httpmock v1.0.5
	github.com/prometheus/client_golang v0.9.3-0.20190127221311-3c4408c8b829
	github.com/prometheus/procfs v0.0.0-20190227231451-bbced9601137 // indirect
	github.com/sirupsen/logrus v1.6.0
	github.com/stretchr/testify v1.6.1
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/resty.v1 v1.12.0
	k8s.io/api v0.19.0
	k8s.io/apimachinery v0.19.0
	k8s.io/client-go v0.19.0
	k8s.io/klog v1.0.0 // indirect
	k8s.io/metrics v0.19.0
	sigs.k8s.io/structured-merge-diff/v2 v2.0.1 // indirect
)

replace k8s.io/klog => github.com/mikkeloscar/knolog v0.0.0-20190326191552-80742771eb6b

go 1.13
