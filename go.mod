module github.com/zalando-incubator/es-operator

require (
	github.com/cenk/backoff v2.2.1+incompatible
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/jarcoal/httpmock v1.1.0
	github.com/prometheus/client_golang v1.11.0
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/testify v1.7.0
	golang.org/x/tools v0.1.1 // indirect
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/resty.v1 v1.12.0
	k8s.io/api v0.21.5
	k8s.io/apiextensions-apiserver v0.21.5 // indirect
	k8s.io/apimachinery v0.21.5
	k8s.io/client-go v0.21.5
	k8s.io/code-generator v0.21.5
	k8s.io/metrics v0.21.5
	sigs.k8s.io/controller-tools v0.4.1-0.20200911221209-6c9ddb17dfd0
)

replace k8s.io/klog => github.com/mikkeloscar/knolog v0.0.0-20190326191552-80742771eb6b

go 1.15
