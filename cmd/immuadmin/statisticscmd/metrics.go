/*
Copyright 2019-2020 vChain, Inc.

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

package statisticscmd

import (
	"time"

	dto "github.com/prometheus/client_model/go"
)

var readers = map[string]bool{
	"ByIndex":     true,
	"ByIndexSV":   true,
	"Consistency": true,
	"Count":       true,
	"CurrentRoot": true,
	"Dump":        true,
	"Get":         true,
	"GetBatch":    true,
	"GetBatchSV":  true,
	"GetSV":       true,
	"Health":      true,
	"History":     true,
	"HistorySV":   true,
	"IScan":       true,
	"IScanSV":     true,
	"Inclusion":   true,
	"Login":       true,
	"SafeGet":     true,
	"SafeGetSV":   true,
	"Scan":        true,
	"ScanSV":      true,
	"ZScan":       true,
	"ZScanSV":     true,
}

var writers = map[string]bool{
	"Reference":     true,
	"SafeReference": true,
	"SafeSet":       true,
	"SafeSetSV":     true,
	"SafeZAdd":      true,
	"Set":           true,
	"SetBatch":      true,
	"SetBatchSV":    true,
	"SetSV":         true,
	"ZAdd":          true,
}

type rpcDuration struct {
	method        string
	counter       uint64
	totalDuration float64
	avgDuration   float64
}

type dbInfo struct {
	name        string
	lsmBytes    uint64
	vlogBytes   uint64
	totalBytes  uint64
	nbEntries   uint64
	uptimeHours float64
}

type operations struct {
	counter     uint64
	duration    float64
	avgDuration float64
}

type memstats struct {
	sysBytes        uint64
	heapAllocBytes  uint64
	heapIdleBytes   uint64
	heapInUseBytes  uint64
	stackInUseBytes uint64
}

type metrics struct {
	durationRPCsByMethod map[string]rpcDuration
	reads                operations
	writes               operations
	nbClients            int
	nbRPCsPerClient      map[string]uint64
	lastMsgAtPerClient   map[string]uint64
	db                   dbInfo
	memstats             memstats
}

func (ms *metrics) clientsActiveDuringLastHour() *map[string]time.Time {
	r := map[string]time.Time{}
	for ip, lastMsgAt := range ms.lastMsgAtPerClient {
		t := time.Unix(int64(lastMsgAt), 0)
		ago := time.Since(t)
		if ago.Hours() < 1 {
			r[ip] = t
		}
	}
	return &r
}

func (ms *metrics) populateFrom(metricsFamilies *map[string]*dto.MetricFamily) {
	ms.withDBInfo(metricsFamilies)
	ms.withClients(metricsFamilies)
	ms.withDuration(metricsFamilies)
	ms.withMemStats(metricsFamilies)
}

func (ms *metrics) withClients(metricsFamilies *map[string]*dto.MetricFamily) {
	ms.nbRPCsPerClient = map[string]uint64{}
	clientsMetrics := (*metricsFamilies)["immudb_number_of_rpcs_per_client"].GetMetric()
	ms.nbClients = len(clientsMetrics)
	for _, m := range clientsMetrics {
		var ip string
		for _, labelPair := range m.GetLabel() {
			if labelPair.GetName() == "ip" {
				ip = labelPair.GetValue()
				break
			}
		}
		ms.nbRPCsPerClient[ip] = uint64(m.GetCounter().GetValue())
	}

	ms.lastMsgAtPerClient = map[string]uint64{}
	lastMsgAtMetrics := (*metricsFamilies)["immudb_clients_last_message_at_unix_seconds"].GetMetric()
	for _, m := range lastMsgAtMetrics {
		var ip string
		for _, labelPair := range m.GetLabel() {
			if labelPair.GetName() == "ip" {
				ip = labelPair.GetValue()
				break
			}
		}
		ms.lastMsgAtPerClient[ip] = uint64(m.GetGauge().GetValue())
	}
}

func (ms *metrics) withDBInfo(metricsFamilies *map[string]*dto.MetricFamily) {
	lsmSizeMetric := (*metricsFamilies)["immudb_lsm_size_bytes"].GetMetric()[0]
	lsmBytes := lsmSizeMetric.GetUntyped().GetValue()
	vlogBytes := (*metricsFamilies)["immudb_vlog_size_bytes"].GetMetric()[0].GetUntyped().GetValue()
	ms.db.lsmBytes = uint64(lsmBytes)
	ms.db.vlogBytes = uint64(vlogBytes)
	ms.db.totalBytes = uint64(lsmBytes + vlogBytes)
	for _, labelPair := range lsmSizeMetric.GetLabel() {
		if labelPair.GetName() == "database" {
			ms.db.name = labelPair.GetValue()
			break
		}
	}
	ms.db.nbEntries = uint64((*metricsFamilies)["immudb_number_of_stored_entries"].GetMetric()[0].GetCounter().GetValue())
	ms.db.uptimeHours = (*metricsFamilies)["immudb_uptime_hours"].GetMetric()[0].GetCounter().GetValue()
}

func (ms *metrics) withDuration(metricsFamilies *map[string]*dto.MetricFamily) {
	ms.durationRPCsByMethod = map[string]rpcDuration{}
	for _, m := range (*metricsFamilies)["grpc_server_handling_seconds"].GetMetric() {
		var method string
		for _, labelPair := range m.GetLabel() {
			if labelPair.GetName() == "grpc_method" {
				method = labelPair.GetValue()
				break
			}
		}
		h := m.GetHistogram()
		c := h.GetSampleCount()
		td := h.GetSampleSum()
		var ad float64
		if c != 0 {
			ad = td / float64(c)
		}
		d := rpcDuration{
			method:        method,
			counter:       c,
			totalDuration: td,
			avgDuration:   ad,
		}
		ms.durationRPCsByMethod[method] = d
		if _, ok := readers[method]; ok {
			ms.reads.counter++
			ms.reads.duration += d.avgDuration
		}
		if _, ok := writers[method]; ok {
			ms.writes.counter += d.counter
			ms.writes.duration += d.totalDuration
		}
	}
	if ms.reads.counter > 0 {
		ms.reads.avgDuration = ms.reads.duration / float64(ms.reads.counter)
	}
	if ms.writes.counter > 0 {
		ms.writes.avgDuration = ms.writes.duration / float64(ms.writes.counter)
	}
}

func (ms *metrics) withMemStats(metricsFamilies *map[string]*dto.MetricFamily) {
	if sysBytesMetric := (*metricsFamilies)["go_memstats_sys_bytes"]; sysBytesMetric != nil {
		ms.memstats.sysBytes = uint64(*sysBytesMetric.GetMetric()[0].GetGauge().Value)
	}
	if heapAllocMetric := (*metricsFamilies)["go_memstats_heap_alloc_bytes"]; heapAllocMetric != nil {
		ms.memstats.heapAllocBytes = uint64(*heapAllocMetric.GetMetric()[0].GetGauge().Value)
	}
	if heapIdleMetric := (*metricsFamilies)["go_memstats_heap_idle_bytes"]; heapIdleMetric != nil {
		ms.memstats.heapIdleBytes = uint64(*heapIdleMetric.GetMetric()[0].GetGauge().Value)
	}
	if heapInUseMetric := (*metricsFamilies)["go_memstats_heap_inuse_bytes"]; heapInUseMetric != nil {
		ms.memstats.heapInUseBytes = uint64(*heapInUseMetric.GetMetric()[0].GetGauge().Value)
	}
	if stackInUseMetric := (*metricsFamilies)["go_memstats_stack_inuse_bytes"]; stackInUseMetric != nil {
		ms.memstats.stackInUseBytes = uint64(*stackInUseMetric.GetMetric()[0].GetGauge().Value)
	}
}
