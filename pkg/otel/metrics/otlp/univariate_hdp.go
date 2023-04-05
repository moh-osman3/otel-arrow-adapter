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
	"github.com/apache/arrow/go/v12/arrow/array"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	arrowutils "github.com/f5/otel-arrow-adapter/pkg/arrow"
	otlp "github.com/f5/otel-arrow-adapter/pkg/otel/common/otlp"
	"github.com/f5/otel-arrow-adapter/pkg/otel/constants"
	"github.com/f5/otel-arrow-adapter/pkg/werror"
)

type UnivariateHistogramDataPointIds struct {
	Id                int
	Attributes        *otlp.AttributeIds
	StartTimeUnixNano int
	TimeUnixNano      int
	Count             int
	Sum               int
	BucketCounts      int // List of uint64
	ExplicitBounds    int // List of float64
	Exemplars         *ExemplarIds
	Flags             int
	Min               int
	Max               int
}

func NewUnivariateHistogramDataPointIds(parentDT *arrow.StructType) (*UnivariateHistogramDataPointIds, error) {
	id, hdpDT, err := arrowutils.ListOfStructsFieldIDFromStruct(parentDT, constants.DataPoints)
	if err != nil {
		return nil, werror.Wrap(err)
	}

	attributes, err := otlp.NewAttributeIds(hdpDT)
	if err != nil {
		return nil, werror.Wrap(err)
	}

	startTimeUnixNanoId, _ := arrowutils.FieldIDFromStruct(hdpDT, constants.StartTimeUnixNano)
	timeUnixNanoId, _ := arrowutils.FieldIDFromStruct(hdpDT, constants.TimeUnixNano)
	countId, _ := arrowutils.FieldIDFromStruct(hdpDT, constants.HistogramCount)
	sumId, _ := arrowutils.FieldIDFromStruct(hdpDT, constants.HistogramSum)
	bucketCountsId, _ := arrowutils.FieldIDFromStruct(hdpDT, constants.HistogramBucketCounts)
	explicitBoundsId, _ := arrowutils.FieldIDFromStruct(hdpDT, constants.HistogramExplicitBounds)

	exemplars, err := NewExemplarIds(hdpDT)
	if err != nil {
		return nil, werror.Wrap(err)
	}

	flagsId, _ := arrowutils.FieldIDFromStruct(hdpDT, constants.Flags)
	minId, _ := arrowutils.FieldIDFromStruct(hdpDT, constants.HistogramMin)
	maxId, _ := arrowutils.FieldIDFromStruct(hdpDT, constants.HistogramMax)

	return &UnivariateHistogramDataPointIds{
		Id:                id,
		Attributes:        attributes,
		StartTimeUnixNano: startTimeUnixNanoId,
		TimeUnixNano:      timeUnixNanoId,
		Count:             countId,
		Sum:               sumId,
		BucketCounts:      bucketCountsId,
		ExplicitBounds:    explicitBoundsId,
		Exemplars:         exemplars,
		Flags:             flagsId,
		Min:               minId,
		Max:               maxId,
	}, nil
}

func AppendUnivariateHistogramDataPointInto(hdpSlice pmetric.HistogramDataPointSlice, hdp *arrowutils.ListOfStructs, ids *UnivariateHistogramDataPointIds, smdata *SharedData, mdata *SharedData) error {
	if hdp == nil {
		return nil
	}

	for hdpIdx := hdp.Start(); hdpIdx < hdp.End(); hdpIdx++ {
		hdpVal := hdpSlice.AppendEmpty()

		if hdp.IsNull(hdpIdx) {
			continue
		}

		attrs := hdpVal.Attributes()
		if err := otlp.AppendAttributesInto(attrs, hdp.Array(), hdpIdx, ids.Attributes); err != nil {
			return werror.Wrap(err)
		}
		smdata.Attributes.Range(func(k string, v pcommon.Value) bool {
			v.CopyTo(attrs.PutEmpty(k))
			return true
		})
		mdata.Attributes.Range(func(k string, v pcommon.Value) bool {
			v.CopyTo(attrs.PutEmpty(k))
			return true
		})

		if smdata.StartTime != nil {
			hdpVal.SetStartTimestamp(*smdata.StartTime)
		} else {
			if mdata.StartTime != nil {
				hdpVal.SetStartTimestamp(*mdata.StartTime)
			} else {
				startTimeUnixNano, err := hdp.TimestampFieldByID(ids.StartTimeUnixNano, hdpIdx)
				if err != nil {
					return werror.Wrap(err)
				}
				hdpVal.SetStartTimestamp(pcommon.Timestamp(startTimeUnixNano))
			}
		}

		if smdata.Time != nil {
			hdpVal.SetTimestamp(*smdata.Time)
		} else {
			if mdata.Time != nil {
				hdpVal.SetTimestamp(*mdata.Time)
			} else {
				timeUnixNano, err := hdp.TimestampFieldByID(ids.TimeUnixNano, hdpIdx)
				if err != nil {
					return werror.Wrap(err)
				}
				hdpVal.SetTimestamp(pcommon.Timestamp(timeUnixNano))
			}
		}

		count, err := hdp.U64FieldByID(ids.Count, hdpIdx)
		if err != nil {
			return werror.Wrap(err)
		}
		hdpVal.SetCount(count)

		sum, err := hdp.F64OrNilFieldByID(ids.Sum, hdpIdx)
		if err != nil {
			return werror.Wrap(err)
		}
		if sum != nil {
			hdpVal.SetSum(*sum)
		}

		bucketCounts, start, end, err := hdp.ListValuesById(hdpIdx, ids.BucketCounts)
		if err != nil {
			return werror.Wrap(err)
		}
		if values, ok := bucketCounts.(*array.Uint64); ok {
			bucketCountsSlice := hdpVal.BucketCounts()
			bucketCountsSlice.EnsureCapacity(end - start)
			for i := start; i < end; i++ {
				bucketCountsSlice.Append(values.Value(i))
			}
		} else {
			return werror.Wrap(ErrNotArrayUint64)
		}

		explicitBounds, start, end, err := hdp.ListValuesById(hdpIdx, ids.ExplicitBounds)
		if err != nil {
			return werror.Wrap(err)
		}
		if values, ok := explicitBounds.(*array.Float64); ok {
			explicitBoundsSlice := hdpVal.ExplicitBounds()
			explicitBoundsSlice.EnsureCapacity(end - start)
			for i := start; i < end; i++ {
				explicitBoundsSlice.Append(values.Value(i))
			}
		} else {
			return werror.Wrap(ErrNotArrayFloat64)
		}

		exemplars, err := hdp.ListOfStructsById(hdpIdx, ids.Exemplars.Id)
		if exemplars != nil && err == nil {
			if err := AppendExemplarsInto(hdpVal.Exemplars(), exemplars, hdpIdx, ids.Exemplars); err != nil {
				return werror.Wrap(err)
			}
		} else if err != nil {
			return werror.Wrap(err)
		}

		flags, err := hdp.U32FieldByID(ids.Flags, hdpIdx)
		if err != nil {
			return werror.Wrap(err)
		}
		hdpVal.SetFlags(pmetric.DataPointFlags(flags))

		min, err := hdp.F64OrNilFieldByID(ids.Min, hdpIdx)
		if err != nil {
			return werror.Wrap(err)
		}
		if min != nil {
			hdpVal.SetMin(*min)
		}

		max, err := hdp.F64OrNilFieldByID(ids.Max, hdpIdx)
		if err != nil {
			return werror.Wrap(err)
		}
		if max != nil {
			hdpVal.SetMax(*max)
		}
	}

	return nil
}
