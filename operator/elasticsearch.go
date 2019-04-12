package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime"

	log "github.com/sirupsen/logrus"
	zv1 "github.com/zalando-incubator/es-operator/pkg/apis/zalando.org/v1"
	"github.com/zalando-incubator/es-operator/pkg/clientset"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	pv1beta1 "k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/cache"
	kube_record "k8s.io/client-go/tools/record"
)

const (
	esDataSetLabelKey                       = "es-operator-dataset"
	esOperatorAnnotationKey                 = "es-operator.zalando.org/operator"
	esScalingOperationKey                   = "es-operator.zalando.org/current-scaling-operation"
	defaultElasticsearchDataSetEndpointPort = 9200
)

type ElasticsearchOperator struct {
	logger                *log.Entry
	kube                  *clientset.Clientset
	interval              time.Duration
	autoscalerInterval    time.Duration
	metricsInterval       time.Duration
	priorityNodeSelectors labels.Set
	operatorID            string
	namespace             string
	clusterDNSZone        string
	elasticsearchEndpoint *url.URL
	operating             map[types.UID]operatingEntry
	sync.Mutex
	recorder kube_record.EventRecorder
}

type operatingEntry struct {
	cancel context.CancelFunc
	doneCh <-chan struct{}
	logger *log.Entry
}

// NewElasticsearchOperator initializes a new ElasticsearchDataSet operator instance.
func NewElasticsearchOperator(
	client *clientset.Clientset,
	priorityNodeSelectors map[string]string,
	interval,
	autoscalerInterval time.Duration,
	operatorID,
	namespace,
	clusterDNSZone string,
	elasticsearchEndpoint *url.URL,
) *ElasticsearchOperator {
	return &ElasticsearchOperator{
		logger: log.WithFields(
			log.Fields{
				"operator": "elasticsearch",
			},
		),
		kube:                  client,
		interval:              interval,
		autoscalerInterval:    autoscalerInterval,
		metricsInterval:       60 * time.Second,
		priorityNodeSelectors: labels.Set(priorityNodeSelectors),
		operatorID:            operatorID,
		namespace:             namespace,
		clusterDNSZone:        clusterDNSZone,
		elasticsearchEndpoint: elasticsearchEndpoint,
		operating:             make(map[types.UID]operatingEntry),
		recorder:              createEventRecorder(client),
	}
}

// Run runs the main loop of the operator.
func (o *ElasticsearchOperator) Run(ctx context.Context) {
	go o.collectMetrics(ctx)
	go o.runAutoscaler(ctx)

	// run EDS watcher
	o.runWatch(ctx)
	<-ctx.Done()
	o.logger.Info("Terminating main operator loop.")
}

// Run setups up a shared informer for listing and watching changes to pods and
// starts listening for events.
func (o *ElasticsearchOperator) runWatch(ctx context.Context) {
	informer := cache.NewSharedIndexInformer(
		cache.NewListWatchFromClient(
			o.kube.ZalandoV1().RESTClient(),
			"elasticsearchdatasets",
			o.namespace, fields.Everything(),
		),
		&zv1.ElasticsearchDataSet{},
		0, // skip resync
		cache.Indexers{},
	)

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    o.add,
		UpdateFunc: o.update,
		DeleteFunc: o.del,
	})

	go informer.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		log.Errorf("Timed out waiting for caches to sync")
		return
	}

	log.Info("Synced ElasticsearchDataSet watcher")
}

// add is the handler for an EDS getting added.
func (o *ElasticsearchOperator) add(obj interface{}) {
	eds, ok := obj.(*zv1.ElasticsearchDataSet)
	if !ok {
		log.Errorf("Failed to get EDS object")
		return
	}

	err := o.operateEDS(eds, false)
	if err != nil {
		log.Errorf("Add failed, this is bad!: %v", err)
	}
}

// update is the handler for an EDS getting updated.
func (o *ElasticsearchOperator) update(oldObj, newObj interface{}) {
	newEDS, ok := newObj.(*zv1.ElasticsearchDataSet)
	if !ok {
		log.Errorf("Failed to get EDS object")
		return
	}

	err := o.operateEDS(newEDS, false)
	if err != nil {
		log.Errorf("Add failed, this is bad!: %v", err)
	}
}

// del is the handler for an EDS getting deleted.
func (o *ElasticsearchOperator) del(obj interface{}) {
	eds, ok := obj.(*zv1.ElasticsearchDataSet)
	if !ok {
		log.Errorf("Failed to get EDS object")
		return
	}

	err := o.operateEDS(eds, true)
	if err != nil {
		log.Errorf("Add failed, this is bad!: %v", err)
	}
}

// collectMetrics collects metrics for all the managed EDS resources.
// The metrics are stored in the coresponding ElasticsearchMetricSet and used
// by the autoscaler for scaling EDS.
func (o *ElasticsearchOperator) collectMetrics(ctx context.Context) {
	nextCheck := time.Now().Add(-o.metricsInterval)

	for {
		o.logger.Debug("Collecting metrics")
		select {
		case <-time.After(time.Until(nextCheck)):
			nextCheck = time.Now().Add(o.metricsInterval)

			resources, err := o.collectResources()
			if err != nil {
				o.logger.Error(err)
				continue
			}

			for _, es := range resources {
				if es.ElasticsearchDataSet.Spec.Scaling != nil && es.ElasticsearchDataSet.Spec.Scaling.Enabled {
					metrics := &ElasticsearchMetricsCollector{
						kube:   o.kube,
						logger: log.WithFields(log.Fields{"collector": "metrics"}),
						es:     *es,
					}
					err := metrics.collectMetrics()
					if err != nil {
						o.logger.Error(err)
						continue
					}
				}
			}
		case <-ctx.Done():
			o.logger.Info("Terminating metrics collector loop.")
			return
		}
	}
}

// runAutoscaler runs the EDS autoscaler which checks at an interval if any
// EDS resources needs to be autoscaled. If autoscaling is needed this will be
// indicated on the EDS with the
// 'es-operator.zalando.org/current-scaling-operation' annotation. The
// annotation indicates the desired scaling which will be reconciled by the
// operator.
func (o *ElasticsearchOperator) runAutoscaler(ctx context.Context) {
	nextCheck := time.Now().Add(-o.autoscalerInterval)

	for {
		o.logger.Debug("Checking autoscaling")
		select {
		case <-time.After(time.Until(nextCheck)):
			nextCheck = time.Now().Add(o.autoscalerInterval)

			resources, err := o.collectResources()
			if err != nil {
				o.logger.Error(err)
				continue
			}

			for _, es := range resources {
				if es.ElasticsearchDataSet.Spec.Scaling != nil && es.ElasticsearchDataSet.Spec.Scaling.Enabled {
					endpoint := o.getElasticsearchEndpoint(es.ElasticsearchDataSet)

					client := &ESClient{
						Endpoint: endpoint,
					}

					err := o.scaleEDS(es.ElasticsearchDataSet, es, client)
					if err != nil {
						o.logger.Error(err)
						continue
					}
				}
			}
		case <-ctx.Done():
			o.logger.Info("Terminating autoscaler loop.")
			return
		}
	}
}

type EDSResource struct {
	eds      *zv1.ElasticsearchDataSet
	kube     *clientset.Clientset
	esClient *ESClient
	recorder kube_record.EventRecorder
}

func (r *EDSResource) Name() string {
	return r.eds.Name
}

func (r *EDSResource) Namespace() string {
	return r.eds.Namespace
}

func (r *EDSResource) APIVersion() string {
	return r.eds.APIVersion
}

func (r *EDSResource) Kind() string {
	return r.eds.Kind
}

func (r *EDSResource) Labels() map[string]string {
	return r.eds.Labels
}

func (r *EDSResource) LabelSelector() map[string]string {
	return map[string]string{esDataSetLabelKey: r.Name()}
}

func (r *EDSResource) Generation() int64 {
	return r.eds.Generation
}

func (r *EDSResource) UID() types.UID {
	return r.eds.UID
}

func (r *EDSResource) Replicas() int32 {
	return edsReplicas(r.eds)
}

func (r *EDSResource) PodTemplateSpec() *v1.PodTemplateSpec {
	return r.eds.Spec.Template.DeepCopy()
}

func (r *EDSResource) VolumeClaimTemplates() []v1.PersistentVolumeClaim {
	return r.eds.Spec.VolumeClaimTemplates
}

func (r *EDSResource) EnsureResources() error {
	// ensure PDB
	err := r.ensurePodDisruptionBudget()
	if err != nil {
		return err
	}

	// ensure service
	err = r.ensureService()
	if err != nil {
		return err
	}

	return nil
}

func (r *EDSResource) Self() runtime.Object {
	return r.eds
}

// ensurePodDisruptionBudget creates a PodDisruptionBudget for the
// ElasticsearchDataSet if it doesn't already exist.
func (r *EDSResource) ensurePodDisruptionBudget() error {
	var pdb *pv1beta1.PodDisruptionBudget
	var err error

	pdb, err = r.kube.PolicyV1beta1().PodDisruptionBudgets(r.eds.Namespace).Get(r.eds.Name, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		pdb = nil
	}

	// check if owner
	if pdb != nil && !isOwnedReference(r, pdb.ObjectMeta) {
		return fmt.Errorf(
			"PodDisruptionBudget %s/%s is not owned by the ElasticsearchDataSet %s/%s",
			pdb.Namespace, pdb.Name,
			r.eds.Namespace, r.eds.Name,
		)
	}

	matchLabels := r.LabelSelector()

	createPDB := false

	if pdb == nil {
		createPDB = true
		maxUnavailable := intstr.FromInt(0)
		pdb = &pv1beta1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      r.eds.Name,
				Namespace: r.eds.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: r.eds.APIVersion,
						Kind:       r.eds.Kind,
						Name:       r.eds.Name,
						UID:        r.eds.UID,
					},
				},
			},
			Spec: pv1beta1.PodDisruptionBudgetSpec{
				MaxUnavailable: &maxUnavailable,
			},
		}
	}

	pdb.Labels = r.eds.Labels
	pdb.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: matchLabels,
	}

	if createPDB {
		var err error
		_, err = r.kube.PolicyV1beta1().PodDisruptionBudgets(pdb.Namespace).Create(pdb)
		if err != nil {
			return err
		}
		r.recorder.Event(r.eds, v1.EventTypeNormal, "CreatedPDB", fmt.Sprintf(
			"Created PodDisruptionBudget '%s/%s' for ElasticsearchDataSet",
			pdb.Namespace, pdb.Name,
		))
	}

	return nil
}

// ensureService creates a Service for the ElasticsearchDataSet if it doesn't
// already exist.
func (r *EDSResource) ensureService() error {
	var svc *v1.Service
	var err error

	svc, err = r.kube.CoreV1().Services(r.eds.Namespace).Get(r.eds.Name, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		svc = nil
	}

	// check if owner
	if svc != nil && !isOwnedReference(r, svc.ObjectMeta) {
		return fmt.Errorf(
			"the Service '%s/%s' is not owned by the ElasticsearchDataSet '%s/%s'",
			svc.Namespace, svc.Name,
			r.eds.Namespace, r.eds.Name,
		)
	}

	matchLabels := r.LabelSelector()

	createService := false

	if svc == nil {
		createService = true
		svc = &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      r.eds.Name,
				Namespace: r.eds.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: r.eds.APIVersion,
						Kind:       r.eds.Kind,
						Name:       r.eds.Name,
						UID:        r.eds.UID,
					},
				},
			},
			Spec: v1.ServiceSpec{
				Type: v1.ServiceTypeClusterIP,
			},
		}
	}

	svc.Labels = r.eds.Labels
	svc.Spec.Selector = matchLabels
	// TODO: derive port from EDS
	svc.Spec.Ports = []v1.ServicePort{
		{
			Name:       "elasticsearch",
			Protocol:   v1.ProtocolTCP,
			Port:       defaultElasticsearchDataSetEndpointPort,
			TargetPort: intstr.FromInt(defaultElasticsearchDataSetEndpointPort),
		},
	}

	if createService {
		var err error
		_, err = r.kube.CoreV1().Services(svc.Namespace).Create(svc)
		if err != nil {
			return err
		}
		r.recorder.Event(r.eds, v1.EventTypeNormal, "CreatedService", fmt.Sprintf(
			"Created Service '%s/%s' for ElasticsearchDataSet",
			svc.Namespace, svc.Name,
		))
	}

	return nil
}

// Drain drains a pod for Elasticsearch data.
func (r *EDSResource) Drain(ctx context.Context, pod *v1.Pod) error {
	return r.esClient.Drain(ctx, pod)
}

// PreScaleDownHook ensures that the IndexReplicas is set as defined in the EDS
// 'scaling-operation' annotation prior to scaling down the internal
// StatefulSet.
func (r *EDSResource) PreScaleDownHook(ctx context.Context) error {
	return r.applyScalingOperation()
}

// OnStableReplicasHook ensures that the indexReplicas is set as defined in the
// EDS scaling-operation annotation.
func (r *EDSResource) OnStableReplicasHook(ctx context.Context) error {
	err := r.applyScalingOperation()
	if err != nil {
		return err
	}

	// cleanup state in ES
	return r.esClient.Cleanup(ctx)
}

// UpdateStatus updates the status of the EDS to set the current replicas from
// StatefulSet and updating the observedGeneration.
func (r *EDSResource) UpdateStatus(sts *appsv1.StatefulSet) error {
	observedGeneration := int64(0)
	if r.eds.Status.ObservedGeneration != nil {
		observedGeneration = *r.eds.Status.ObservedGeneration
	}

	replicas := int32(0)
	if sts.Spec.Replicas != nil {
		replicas = *sts.Spec.Replicas
	}

	if r.eds.Generation != observedGeneration ||
		r.eds.Status.Replicas != replicas {
		r.eds.Status.Replicas = replicas
		r.eds.Status.ObservedGeneration = &r.eds.Generation
		var err error
		r.eds, err = r.kube.ZalandoV1().ElasticsearchDataSets(r.eds.Namespace).UpdateStatus(r.eds)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *EDSResource) applyScalingOperation() error {
	operation, err := edsScalingOperation(r.eds)
	if err != nil {
		return err
	}

	if operation != nil && operation.ScalingDirection != NONE {
		err = r.esClient.UpdateIndexSettings(operation.IndexReplicas)
		if err != nil {
			return err
		}
		err = r.removeScalingOperationAnnotation()
		if err != nil {
			return err
		}
	}
	return nil
}

// removeScalingOperationAnnotation removes the 'scaling-operation' annotation
// from the EDS.
// If the annotation is already gone, this is a no-op.
func (r *EDSResource) removeScalingOperationAnnotation() error {
	if _, ok := r.eds.Annotations[esScalingOperationKey]; ok {
		delete(r.eds.Annotations, esScalingOperationKey)
		_, err := r.kube.ZalandoV1().
			ElasticsearchDataSets(r.eds.Namespace).
			Update(r.eds)
		if err != nil {
			return fmt.Errorf("failed to remove 'scaling-operating' annotation from EDS: %v", err)
		}
	}
	return nil
}

func (o *ElasticsearchOperator) operateEDS(eds *zv1.ElasticsearchDataSet, deleted bool) error {
	// set TypeMeta manually because of this bug:
	// https://github.com/kubernetes/client-go/issues/308
	eds.APIVersion = "zalando.org/v1"
	eds.Kind = "ElasticsearchDataSet"

	// insert into operating
	o.Lock()
	defer o.Unlock()

	// restart if already being operated on.
	if entry, ok := o.operating[eds.UID]; ok {
		entry.cancel()
		// wait for previous operation to terminate
		entry.logger.Infof("Waiting for operation to stop")
		<-entry.doneCh
	}

	if deleted {
		return nil
	}

	if !o.hasOwnership(eds) {
		o.logger.Infof("Skipping EDS %s/%s, not owned by the operator", eds.Namespace, eds.Name)
		return nil
	}

	doneCh := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())

	logger := log.WithFields(log.Fields{
		"eds":       eds.Name,
		"namespace": eds.Namespace,
	})
	// add to operating table
	o.operating[eds.UID] = operatingEntry{
		cancel: cancel,
		doneCh: doneCh,
		logger: logger,
	}

	endpoint := o.getElasticsearchEndpoint(eds)

	// TODO: abstract this
	client := &ESClient{
		Endpoint: endpoint,
	}

	operator := &Operator{
		kube:                  o.kube,
		priorityNodeSelectors: o.priorityNodeSelectors,
		interval:              o.interval,
		logger:                logger,
		recorder:              o.recorder,
	}

	rs := &EDSResource{
		eds:      eds,
		kube:     o.kube,
		esClient: client, // TODO: think about not setting this twice
		recorder: o.recorder,
	}

	go operator.Run(ctx, doneCh, rs)

	return nil
}

// hasOwnership returns true if the operator is the "owner" of the EDS.
// Whether it's owner is determined by the value of the
// 'es-operator.zalando.org/operator' annotation. If the value
// matches the operatorID then it owns it, or if the operatorID is
// "" and there's no annotation set.
func (o *ElasticsearchOperator) hasOwnership(eds *zv1.ElasticsearchDataSet) bool {
	if eds.Annotations != nil {
		if owner, ok := eds.Annotations[esOperatorAnnotationKey]; ok {
			return owner == o.operatorID
		}
	}
	return o.operatorID == ""
}

func (o *ElasticsearchOperator) getElasticsearchEndpoint(eds *zv1.ElasticsearchDataSet) *url.URL {
	if o.elasticsearchEndpoint != nil {
		return o.elasticsearchEndpoint
	}

	// TODO: discover port from EDS
	return &url.URL{
		Scheme: "http",
		Host: fmt.Sprintf(
			"%s.%s.svc.%s:%d",
			eds.Name,
			eds.Namespace,
			o.clusterDNSZone,
			defaultElasticsearchDataSetEndpointPort,
		),
	}
}

type ESResource struct {
	ElasticsearchDataSet *zv1.ElasticsearchDataSet
	StatefulSet          *appsv1.StatefulSet
	MetricSet            *zv1.ElasticsearchMetricSet
	Pods                 []v1.Pod
}

// Replicas returns the desired node replicas of an ElasticsearchDataSet
// In case it was not specified, it will return '1'.
func (es *ESResource) Replicas() int32 {
	return edsReplicas(es.ElasticsearchDataSet)
}

// edsReplicas returns the desired node replicas of an ElasticsearchDataSet
// In case it was not specified, it will return '1'.
func edsReplicas(eds *zv1.ElasticsearchDataSet) int32 {
	scaling := eds.Spec.Scaling
	if scaling == nil || !scaling.Enabled {
		if eds.Spec.Replicas == nil {
			return 1
		}
		return *eds.Spec.Replicas
	}
	// initialize with minReplicas
	minReplicas := eds.Spec.Scaling.MinReplicas
	if eds.Spec.Replicas == nil {
		return minReplicas
	}
	currentReplicas := *eds.Spec.Replicas
	return int32(math.Max(float64(currentReplicas), float64(scaling.MinReplicas)))
}

// collectResources collects all the ElasticsearchDataSet resources and there
// corresponding StatefulSets if they exist.
func (o *ElasticsearchOperator) collectResources() (map[types.UID]*ESResource, error) {
	resources := make(map[types.UID]*ESResource)

	edss, err := o.kube.ZalandoV1().ElasticsearchDataSets(o.namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	// create a map of ElasticsearchDataSet clusters to later map the matching
	// StatefulSet.
	for _, eds := range edss.Items {
		eds := eds
		if !o.hasOwnership(&eds) {
			continue
		}

		// set TypeMeta manually because of this bug:
		// https://github.com/kubernetes/client-go/issues/308
		eds.APIVersion = "zalando.org/v1"
		eds.Kind = "ElasticsearchDataSet"

		resources[eds.UID] = &ESResource{
			ElasticsearchDataSet: &eds,
		}
	}

	metricSets, err := o.kube.ZalandoV1().ElasticsearchMetricSets(o.namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	// Map metricSets to the owning ElasticsearchDataSet resource.
	for _, ms := range metricSets.Items {
		ms := ms
		if uid, ok := getOwnerUID(ms.ObjectMeta); ok {
			if er, ok := resources[uid]; ok {
				er.MetricSet = &ms
			}
		}
	}

	// TODO: label filter
	pods, err := o.kube.CoreV1().Pods(o.namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, pod := range pods.Items {
		for _, es := range resources {
			es := es
			// TODO: leaky abstraction
			if v, ok := pod.Labels[esDataSetLabelKey]; ok && v == es.ElasticsearchDataSet.Name {
				es.Pods = append(es.Pods, pod)
				break
			}
		}
	}

	return resources, nil
}

func (o *ElasticsearchOperator) scaleEDS(eds *zv1.ElasticsearchDataSet, es *ESResource, client *ESClient) error {
	err := validateScalingSettings(eds.Spec.Scaling)
	if err != nil {
		o.recorder.Event(eds, v1.EventTypeWarning, "ScalingInvalid", fmt.Sprintf(
			"Scaling settings are invalid: %v", err),
		)
		return err
	}

	// first, try to find an existing annotation and return it
	scalingOperation, err := edsScalingOperation(eds)
	if err != nil {
		return err
	}

	// exit early if the scaling operation is already defined
	if scalingOperation != nil && scalingOperation.ScalingDirection != NONE {
		return nil
	}

	// second, calculate a new EDS scaling operation
	scaling := eds.Spec.Scaling
	name := eds.Name
	namespace := eds.Namespace

	currentReplicas := edsReplicas(eds)
	eds.Spec.Replicas = &currentReplicas
	as := NewAutoScaler(es, o.metricsInterval, client)

	if scaling != nil && scaling.Enabled {
		scalingOperation, err := as.GetScalingOperation()
		if err != nil {
			return err
		}

		// update EDS definition.
		if scalingOperation.NodeReplicas != nil && *scalingOperation.NodeReplicas != currentReplicas {
			now := metav1.Now()
			if *scalingOperation.NodeReplicas > currentReplicas {
				eds.Status.LastScaleUpStarted = &now
			} else {
				eds.Status.LastScaleDownStarted = &now
			}
			log.Infof("Updating last scaling event in EDS '%s/%s'", namespace, name)

			// update status
			eds, err = o.kube.ZalandoV1().ElasticsearchDataSets(eds.Namespace).UpdateStatus(eds)
			if err != nil {
				return err
			}
			eds.Spec.Replicas = scalingOperation.NodeReplicas
		}

		// TODO: move to a function
		jsonBytes, err := json.Marshal(scalingOperation)
		if err != nil {
			return err
		}

		if scalingOperation.ScalingDirection != NONE {
			eds.Annotations[esScalingOperationKey] = string(jsonBytes)

			// persist changes of EDS
			log.Infof("Updating desired scaling for EDS '%s/%s'. New desired replicas: %d. %s", namespace, name, *eds.Spec.Replicas, scalingOperation.Description)
			_, err = o.kube.ZalandoV1().ElasticsearchDataSets(eds.Namespace).Update(eds)
			if err != nil {
				return err
			}
		}

	}

	return nil
}

func getOwnerUID(objectMeta metav1.ObjectMeta) (types.UID, bool) {
	if len(objectMeta.OwnerReferences) == 1 {
		return objectMeta.OwnerReferences[0].UID, true
	}
	return "", false
}

// templateInjectLabels injects labels into a pod template spec.
func templateInjectLabels(template v1.PodTemplateSpec, labels map[string]string) v1.PodTemplateSpec {
	if template.ObjectMeta.Labels == nil {
		template.ObjectMeta.Labels = map[string]string{}
	}

	for key, value := range labels {
		if _, ok := template.ObjectMeta.Labels[key]; !ok {
			template.ObjectMeta.Labels[key] = value
		}
	}
	return template
}

// edsScalingOperation returns the scaling operation read from the
// scaling-operation annotation on the EDS. If no operation is defined it will
// return nil.
func edsScalingOperation(eds *zv1.ElasticsearchDataSet) (*ScalingOperation, error) {
	if op, ok := eds.Annotations[esScalingOperationKey]; ok {
		scalingOperation := &ScalingOperation{}
		err := json.Unmarshal([]byte(op), scalingOperation)
		if err != nil {
			return nil, err
		}
		return scalingOperation, nil
	}
	return nil, nil
}

// validateScalingSettings checks that the scaling settings are valid.
//
// * min values can not be > than the corresponding max values.
// * min can not be less than 0.
// * 'replicas', 'indexReplicas' and 'shardsPerNode' settings should not
//   conflict.
func validateScalingSettings(scaling *zv1.ElasticsearchDataSetScaling) error {
	// don't validate if scaling is not enabled
	if scaling == nil || !scaling.Enabled {
		return nil
	}

	// check that min is not greater than max values
	if scaling.MinReplicas > scaling.MaxReplicas {
		return fmt.Errorf(
			"minReplicas(%d) can't be greater than maxReplicas(%d)",
			scaling.MinReplicas,
			scaling.MaxReplicas,
		)
	}

	if scaling.MinIndexReplicas > scaling.MaxIndexReplicas {
		return fmt.Errorf(
			"minIndexReplicas(%d) can't be greater than maxIndexReplicas(%d)",
			scaling.MinIndexReplicas,
			scaling.MaxIndexReplicas,
		)
	}

	if scaling.MinShardsPerNode > scaling.MaxShardsPerNode {
		return fmt.Errorf(
			"minShardsPerNode(%d) can't be greater than maxShardsPerNode(%d)",
			scaling.MinShardsPerNode,
			scaling.MaxShardsPerNode,
		)
	}

	// ensure that relation between replicas and indexReplicas is valid
	if scaling.MinReplicas < (scaling.MinIndexReplicas + 1) {
		return fmt.Errorf(
			"minReplicas(%d) can not be less than minIndexReplicas(%d)+1",
			scaling.MinReplicas,
			scaling.MinIndexReplicas,
		)
	}

	if scaling.MaxReplicas < (scaling.MaxIndexReplicas + 1) {
		return fmt.Errorf(
			"maxReplicas(%d) can not be less than maxIndexReplicas(%d)+1",
			scaling.MaxReplicas,
			scaling.MaxIndexReplicas,
		)
	}

	return nil
}
