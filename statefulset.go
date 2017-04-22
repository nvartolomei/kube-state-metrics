/*
Copyright 2016 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/context"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	v1beta1 "k8s.io/client-go/pkg/apis/apps/v1beta1"
	"k8s.io/client-go/tools/cache"
)

var (
	descStatefulSetStatusReplicas = prometheus.NewDesc(
		"kube_statefulset_status_replicas",
		"The number of replicas per StatefulSet.",
		[]string{"namespace", "statefulset"}, nil,
	)
	descStatefulSetStatusObservedGeneration = prometheus.NewDesc(
		"kube_statefulset_status_observed_generation",
		"The generation observed by the StatefulSet controller.",
		[]string{"namespace", "statefulset"}, nil,
	)
	descStatefulSetSpecReplicas = prometheus.NewDesc(
		"kube_statefulset_spec_replicas",
		"Number of desired pods for a StatefulSet.",
		[]string{"namespace", "statefulset"}, nil,
	)
	descStatefulSetMetadataGeneration = prometheus.NewDesc(
		"kube_statefulset_metadata_generation",
		"Sequence number representing a specific generation of the desired state.",
		[]string{"namespace", "statefulset"}, nil,
	)
)

type StatefulSetLister func() ([]v1beta1.StatefulSet, error)

func (l StatefulSetLister) List() ([]v1beta1.StatefulSet, error) {
	return l()
}

func RegisterStatefulSetCollector(registry prometheus.Registerer, kubeClient kubernetes.Interface) {
	client := kubeClient.Extensions().RESTClient()
	dslw := cache.NewListWatchFromClient(client, "statefulsets", api.NamespaceAll, nil)
	dsinf := cache.NewSharedInformer(dslw, &v1beta1.StatefulSet{}, resyncPeriod)

	dsLister := StatefulSetLister(func() (statefulsets []v1beta1.StatefulSet, err error) {
		for _, c := range dsinf.GetStore().List() {
			statefulsets = append(statefulsets, *(c.(*v1beta1.StatefulSet)))
		}
		return statefulsets, nil
	})

	registry.MustRegister(&statefulsetCollector{store: dsLister})
	go dsinf.Run(context.Background().Done())
}

type statefulsetStore interface {
	List() (statefulsets []v1beta1.StatefulSet, err error)
}

// statefulsetCollector collects metrics about all statefulsets in the cluster.
type statefulsetCollector struct {
	store statefulsetStore
}

// Describe implements the prometheus.Collector interface.
func (dc *statefulsetCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- descStatefulSetStatusReplicas
	ch <- descStatefulSetStatusObservedGeneration
	ch <- descStatefulSetSpecReplicas
	ch <- descStatefulSetMetadataGeneration
}

// Collect implements the prometheus.Collector interface.
func (dc *statefulsetCollector) Collect(ch chan<- prometheus.Metric) {
	dpls, err := dc.store.List()
	if err != nil {
		glog.Errorf("listing statefulsets failed: %s", err)
		return
	}
	for _, d := range dpls {
		dc.collectStatefulSet(ch, d)
	}
}

func (dc *statefulsetCollector) collectStatefulSet(ch chan<- prometheus.Metric, d v1beta1.StatefulSet) {
	addGauge := func(desc *prometheus.Desc, v float64, lv ...string) {
		lv = append([]string{d.Namespace, d.Name}, lv...)
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, v, lv...)
	}
	addGauge(descStatefulSetStatusReplicas, float64(d.Status.Replicas))
	addGauge(descStatefulSetStatusObservedGeneration, float64(*d.Status.ObservedGeneration))
	addGauge(descStatefulSetSpecReplicas, float64(*d.Spec.Replicas))
	addGauge(descStatefulSetMetadataGeneration, float64(d.ObjectMeta.Generation))
}
