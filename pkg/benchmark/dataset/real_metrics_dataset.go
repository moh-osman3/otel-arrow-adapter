// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dataset

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	// "strconv"
	// "path/filepath"

	"github.com/klauspost/compress/zstd"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/f5/otel-arrow-adapter/pkg/benchmark/stats"
)

// RealMetricsDataset represents a dataset of real metrics read from a Metrics serialized to a binary file.
type RealMetricsDataset struct {
	metrics      []metrics
	sizeInBytes  int
	metricsStats *stats.MetricsStats
}

type metrics struct {
	metric   pmetric.Metric
	resource pmetric.ResourceMetrics
	scope    pmetric.ScopeMetrics
}

type metricReader struct {
	stringReader *bufio.Reader
	unmarshaler  *pmetric.JSONUnmarshaler
	bytesRead    int
}

func (mr *metricReader) readAllMetrics() (pmetric.Metrics, error) {
	metrics := pmetric.NewMetrics()

	for {
		if line, err := mr.stringReader.ReadString('\n'); err != nil {
			fmt.Println(json.Valid([]byte(line)))
			fmt.Println(line)
			// un, _ := strconv.Unquote(line)
			ml, err := mr.unmarshaler.UnmarshalMetrics([]byte(line))
			if err != nil {
				return metrics, err
			}
			for i := 0; i < ml.ResourceMetrics().Len(); i++ {
				rm := metrics.ResourceMetrics().AppendEmpty()
				ml.ResourceMetrics().At(i).CopyTo(rm) 
			} 
			mr.bytesRead += len(line)
		} else { // failed to read line
			if err != nil {
				if errors.Is(err, io.EOF) {
					return metrics, nil
				}
				return metrics, err
			}
		}

	}
}

// NewRealMetricsDataset creates a new RealMetricsDataset from a binary file.
func NewRealMetricsDataset(file *os.File, compression string) *RealMetricsDataset {

	mr := &metricReader{
		unmarshaler: &pmetric.JSONUnmarshaler{},
		bytesRead: 0,
	}

	if compression == "zstd" {
		cr, err := zstd.NewReader(file)
		if err != nil {
			log.Fatal("Failed to create compressed reader")
		}
		mr.stringReader = bufio.NewReader(cr)
	} else { // no compression
		mr.stringReader = bufio.NewReader(file)
	}

	mdata, err := mr.readAllMetrics() 
	fmt.Println("BYTES READ")
	fmt.Println(mr.bytesRead)

	if err != nil {
		log.Fatal("Failed to read lines from file: ", err)
	}

	ds := &RealMetricsDataset{
		metrics:      []metrics{},
		sizeInBytes:  mr.bytesRead, 
		metricsStats: stats.NewMetricsStats(),
	}
	ds.metricsStats.Analyze(mdata)

	for ri := 0; ri < mdata.ResourceMetrics().Len(); ri++ {
		rm := mdata.ResourceMetrics().At(ri)
		for si := 0; si < rm.ScopeMetrics().Len(); si++ {
			sm := rm.ScopeMetrics().At(si)
			for mi := 0; mi < sm.Metrics().Len(); mi++ {
				m := sm.Metrics().At(mi)
				ds.metrics = append(ds.metrics, metrics{metric: m, resource: rm, scope: sm})
			}
		}
	}

	return ds
}

func (d *RealMetricsDataset) SizeInBytes() int {
	return d.sizeInBytes
}

// Len returns the number of metrics in the dataset.
func (d *RealMetricsDataset) Len() int {
	return len(d.metrics)
}

func (d *RealMetricsDataset) ShowStats() {
	println()
	println("Metrics stats:")
	d.metricsStats.ShowStats()
}

// Resize resizes the dataset to the specified max size or do nothing if the current size is already lower than the
// specified max size.
func (d *RealMetricsDataset) Resize(maxSize int) {
	d.metrics = d.metrics[:maxSize]
}

// Metrics returns a subset of metrics from the original dataset.
func (d *RealMetricsDataset) Metrics(offset, size int) []pmetric.Metrics {
	resMetrics := map[pmetric.ResourceMetrics]map[pmetric.ScopeMetrics][]pmetric.Metric{}

	for _, metric := range d.metrics[offset : offset+size] {
		if _, ok := resMetrics[metric.resource]; !ok {
			resMetrics[metric.resource] = map[pmetric.ScopeMetrics][]pmetric.Metric{}
		}

		resMetrics[metric.resource][metric.scope] =
			append(resMetrics[metric.resource][metric.scope], metric.metric)
	}

	request := pmetric.NewMetrics()

	for rm, smm := range resMetrics {
		outRm := request.ResourceMetrics().AppendEmpty()
		rm.Resource().CopyTo(outRm.Resource())

		for sm, ms := range smm {
			outSm := outRm.ScopeMetrics().AppendEmpty()
			sm.Scope().CopyTo(outSm.Scope())

			for _, m := range ms {
				outM := outSm.Metrics().AppendEmpty()
				m.CopyTo(outM)
			}
		}
	}

	return []pmetric.Metrics{request}
}
