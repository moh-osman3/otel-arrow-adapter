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

package otlp

import (
	"github.com/apache/arrow/go/v12/arrow"
	"go.opentelemetry.io/collector/pdata/pmetric"

	arrowutils "github.com/f5/otel-arrow-adapter/pkg/arrow"
	otlp "github.com/f5/otel-arrow-adapter/pkg/otel/common/otlp_old"
	"github.com/f5/otel-arrow-adapter/pkg/otel/constants"
	"github.com/f5/otel-arrow-adapter/pkg/werror"
)

type ResourceMetricsIds struct {
	Id             int
	Resource       *otlp.ResourceIds
	SchemaUrl      int
	ScopeMetricsId int
	ScopeMetrics   *ScopeMetricsIds
}

func NewResourceMetricsIds(schema *arrow.Schema) (*ResourceMetricsIds, error) {
	id, rsDT, err := arrowutils.ListOfStructsFieldIDFromSchema(schema, constants.ResourceMetrics)
	if err != nil {
		return nil, werror.Wrap(err)
	}

	schemaId, _ := arrowutils.FieldIDFromStruct(rsDT, constants.SchemaUrl)

	scopeMetricsId, scopeMetricsDT, err := arrowutils.ListOfStructsFieldIDFromStruct(rsDT, constants.ScopeMetrics)
	if err != nil {
		return nil, werror.Wrap(err)
	}
	scopeMetricsIds, err := NewScopeMetricsIds(scopeMetricsDT)
	if err != nil {
		return nil, werror.Wrap(err)
	}

	resourceIds, err := otlp.NewResourceIds(rsDT)
	if err != nil {
		return nil, werror.Wrap(err)
	}

	return &ResourceMetricsIds{
		Id:             id,
		Resource:       resourceIds,
		SchemaUrl:      schemaId,
		ScopeMetricsId: scopeMetricsId,
		ScopeMetrics:   scopeMetricsIds,
	}, nil
}

func AppendResourceMetricsInto(metrics pmetric.Metrics, record arrow.Record, metricsIds *MetricsIds) error {
	resMetricsSlice := metrics.ResourceMetrics()
	resMetricsCount := int(record.NumRows())

	for metricsIdx := 0; metricsIdx < resMetricsCount; metricsIdx++ {
		arrowResEnts, err := arrowutils.ListOfStructsFromRecord(record, metricsIds.ResourceMetrics.Id, metricsIdx)
		if err != nil {
			return werror.Wrap(err)
		}
		resMetricsSlice.EnsureCapacity(resMetricsSlice.Len() + arrowResEnts.End() - arrowResEnts.Start())

		for resMetricsIdx := arrowResEnts.Start(); resMetricsIdx < arrowResEnts.End(); resMetricsIdx++ {
			resMetrics := resMetricsSlice.AppendEmpty()

			if err = otlp.UpdateResourceWith(resMetrics.Resource(), arrowResEnts, resMetricsIdx, metricsIds.ResourceMetrics.Resource); err != nil {
				return werror.Wrap(err)
			}

			schemaUrl, err := arrowResEnts.StringFieldByID(metricsIds.ResourceMetrics.SchemaUrl, resMetricsIdx)
			if err != nil {
				return werror.Wrap(err)
			}
			resMetrics.SetSchemaUrl(schemaUrl)

			arrowScopeMetrics, err := arrowResEnts.ListOfStructsById(resMetricsIdx, metricsIds.ResourceMetrics.ScopeMetricsId)
			if err != nil {
				return werror.Wrap(err)
			}
			err = UpdateScopeMetricsFrom(resMetrics.ScopeMetrics(), arrowScopeMetrics, metricsIds.ResourceMetrics.ScopeMetrics)
			if err != nil {
				return werror.Wrap(err)
			}
		}
	}

	return nil
}
