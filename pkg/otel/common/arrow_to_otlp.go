/*
 * // Copyright The OpenTelemetry Authors
 * //
 * // Licensed under the Apache License, Version 2.0 (the "License");
 * // you may not use this file except in compliance with the License.
 * // You may obtain a copy of the License at
 * //
 * //       http://www.apache.org/licenses/LICENSE-2.0
 * //
 * // Unless required by applicable law or agreed to in writing, software
 * // distributed under the License is distributed on an "AS IS" BASIS,
 * // WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * // See the License for the specific language governing permissions and
 * // limitations under the License.
 *
 */

package common

import (
	"fmt"
	"strings"

	"github.com/apache/arrow/go/v9/arrow"
	"github.com/apache/arrow/go/v9/arrow/array"

	"github.com/lquerel/otel-arrow-adapter/pkg/air"
	"github.com/lquerel/otel-arrow-adapter/pkg/otel/constants"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

func AttributesId(attrs pcommon.Map) string {
	var attrsId strings.Builder
	attrs.Sort()
	attrsId.WriteString("{")
	attrs.Range(func(k string, v pcommon.Value) bool {
		if attrsId.Len() > 1 {
			attrsId.WriteString(",")
		}
		attrsId.WriteString(k)
		attrsId.WriteString(":")
		attrsId.WriteString(ValueId(v))
		return true
	})
	attrsId.WriteString("}")
	return attrsId.String()
}

// TODO replace this implementation with the one used for traces.
func NewResourceFromOld(record arrow.Record, row int) (pcommon.Resource, error) {
	r := pcommon.NewResource()
	resourceField, resourceArray, err := air.StructFromRecord(record, constants.RESOURCE)
	if err != nil {
		return r, err
	}
	droppedAttributesCount, err := air.U32FromStruct(resourceField, resourceArray, row, constants.DROPPED_ATTRIBUTES_COUNT)
	if err != nil {
		return r, err
	}
	attrField, attrArray, err := air.FieldArrayOfStruct(resourceField, resourceArray, constants.ATTRIBUTES)
	if err != nil {
		return r, err
	}
	if attrField != nil {
		if err = CopyAttributesFrom(r.Attributes(), attrField.Type, attrArray, row); err != nil {
			return r, err
		}
	}
	r.SetDroppedAttributesCount(droppedAttributesCount)
	return r, nil
}

// NewResourceFrom creates a new Resource from the given array and row.
func NewResourceFrom(resList *air.ListOfStructs, row int) (pcommon.Resource, error) {
	r := pcommon.NewResource()
	resDt, resArr, err := resList.StructArray(constants.RESOURCE, row)
	if err != nil {
		return r, err
	}

	// Read dropped attributes count
	droppedAttributesCount, err := air.U32FromStruct(resDt, resArr, row, constants.DROPPED_ATTRIBUTES_COUNT)
	if err != nil {
		return r, err
	}
	r.SetDroppedAttributesCount(droppedAttributesCount)

	// Read attributes
	attrs, err := air.ListOfStructsFromStruct(resDt, resArr, row, constants.ATTRIBUTES)
	if err != nil {
		return r, err
	}
	if attrs != nil {
		err = attrs.CopyAttributesFrom(r.Attributes())
	}

	return r, err
}

func NewScopeFrom(listOfStructs *air.ListOfStructs, row int) (pcommon.InstrumentationScope, error) {
	s := pcommon.NewInstrumentationScope()
	scopeField, scopeArray, err := listOfStructs.StructArray(constants.SCOPE, row)
	if err != nil {
		return s, err
	}
	name, err := air.StringFromStruct(scopeField, scopeArray, row, constants.NAME)
	if err != nil {
		return s, err
	}
	version, err := air.StringFromStruct(scopeField, scopeArray, row, constants.VERSION)
	if err != nil {
		return s, err
	}
	droppedAttributesCount, err := air.U32FromStruct(scopeField, scopeArray, row, constants.DROPPED_ATTRIBUTES_COUNT)
	if err != nil {
		return s, err
	}

	attrs, err := air.ListOfStructsFromStruct(scopeField, scopeArray, row, constants.ATTRIBUTES)
	if err != nil {
		return s, err
	}
	if attrs != nil {
		err = attrs.CopyAttributesFrom(s.Attributes())
	}
	s.SetName(name)
	s.SetVersion(version)
	s.SetDroppedAttributesCount(droppedAttributesCount)
	return s, nil
}

func NewInstrumentationScopeFrom(record arrow.Record, row int, scope string) (pcommon.InstrumentationScope, error) {
	s := pcommon.NewInstrumentationScope()
	scopeField, scopeArray, err := air.StructFromRecord(record, scope)
	if err != nil {
		return s, err
	}
	name, err := air.StringFromStruct(scopeField, scopeArray, row, constants.NAME)
	if err != nil {
		return s, err
	}
	version, err := air.StringFromStruct(scopeField, scopeArray, row, constants.VERSION)
	if err != nil {
		return s, err
	}
	droppedAttributesCount, err := air.U32FromStruct(scopeField, scopeArray, row, constants.DROPPED_ATTRIBUTES_COUNT)
	if err != nil {
		return s, err
	}
	attrField, attrArray, err := air.FieldArrayOfStruct(scopeField, scopeArray, constants.ATTRIBUTES)
	if err != nil {
		return s, err
	}
	if attrField != nil {
		if err = CopyAttributesFrom(s.Attributes(), attrField.Type, attrArray, row); err != nil {
			return s, err
		}
	}
	s.SetName(name)
	s.SetVersion(version)
	s.SetDroppedAttributesCount(droppedAttributesCount)
	return s, nil
}

func ResourceId(r pcommon.Resource) string {
	return AttributesId(r.Attributes()) + "|" + fmt.Sprintf("dac:%d", r.DroppedAttributesCount())
}

func ScopeId(is pcommon.InstrumentationScope) string {
	return "name:" + is.Name() + "|version:" + is.Version() + "|" + AttributesId(is.Attributes()) + "|" + fmt.Sprintf("dac:%d", is.DroppedAttributesCount())
}

func ValueId(v pcommon.Value) string {
	switch v.Type() {
	case pcommon.ValueTypeStr:
		return v.Str()
	case pcommon.ValueTypeInt:
		return fmt.Sprintf("%d", v.Int())
	case pcommon.ValueTypeDouble:
		return fmt.Sprintf("%f", v.Double())
	case pcommon.ValueTypeBool:
		return fmt.Sprintf("%t", v.Bool())
	case pcommon.ValueTypeMap:
		return AttributesId(v.Map())
	case pcommon.ValueTypeBytes:
		return fmt.Sprintf("%x", v.Bytes().AsRaw())
	case pcommon.ValueTypeSlice:
		values := v.Slice()
		valueId := "["
		for i := 0; i < values.Len(); i++ {
			if len(valueId) > 1 {
				valueId += ","
			}
			valueId += ValueId(values.At(i))
		}
		valueId += "]"
		return valueId
	default:
		// includes pcommon.ValueTypeEmpty
		panic("unsupported value type")
	}
}

func CopyAttributesFrom(a pcommon.Map, dt arrow.DataType, arr arrow.Array, row int) error {
	structType, ok := dt.(*arrow.StructType)
	if !ok {
		return fmt.Errorf("attributes is not a struct")
	}
	attrArray := arr.(*array.Struct)
	a.EnsureCapacity(attrArray.NumField())
	for i := 0; i < attrArray.NumField(); i++ {
		valueField := structType.Field(i)

		newV := a.PutEmpty(valueField.Name)

		if err := CopyValueFrom(newV, valueField.Type, attrArray.Field(i), row); err != nil {
			return err
		}
	}
	return nil
}

func CopyValueFrom(dest pcommon.Value, dt arrow.DataType, arr arrow.Array, row int) error {
	switch t := dt.(type) {
	case *arrow.BooleanType:
		v, err := air.BoolFromArray(arr, row)
		if err != nil {
			return err
		}
		dest.SetBool(v)
		return nil
	case *arrow.Float64Type:
		v, err := air.F64FromArray(arr, row)
		if err != nil {
			return err
		}
		dest.SetDouble(v)
		return nil
	case *arrow.Int64Type:
		v, err := air.I64FromArray(arr, row)
		if err != nil {
			return err
		}
		dest.SetInt(v)
		return nil
	case *arrow.StringType:
		v, err := air.StringFromArray(arr, row)
		if err != nil {
			return err
		}
		dest.SetStr(v)
		return nil
	case *arrow.BinaryType:
		v, err := air.BinaryFromArray(arr, row)
		if err != nil {
			return err
		}
		dest.SetEmptyBytes().FromRaw(v)
		return nil
	case *arrow.StructType:
		if err := CopyAttributesFrom(dest.SetEmptyMap(), dt, arr, row); err != nil {
			return err
		}
		return nil
	case *arrow.ListType:
		arrList, ok := arr.(*array.List)
		if !ok {
			return fmt.Errorf("array is not a list")
		}
		if err := SetArrayValue(dest.SetEmptySlice(), arrList, row); err != nil {
			return err
		}
		return nil
	case *arrow.DictionaryType:
		switch t.ValueType.(type) {
		case *arrow.StringType:
			v, err := air.StringFromArray(arr, row)
			if err != nil {
				return err
			}
			dest.SetStr(v)
			return nil
		case *arrow.BinaryType:
			v, err := air.BinaryFromArray(arr, row)
			if err != nil {
				return err
			}
			dest.SetEmptyBytes().FromRaw(v)
			return nil
		default:
			return fmt.Errorf("unsupported dictionary value type %T", t.ValueType)
		}
	default:
		return fmt.Errorf("%T is not a supported value type", t)
	}
}

func SetArrayValue(result pcommon.Slice, arrList *array.List, row int) error {
	start := int(arrList.Offsets()[row])
	end := int(arrList.Offsets()[row+1])
	result.EnsureCapacity(end - start)

	arrItems := arrList.ListValues()
	for ; start < end; start++ {
		v := result.AppendEmpty()
		if arrList.IsNull(start) {
			continue
		}
		if err := CopyValueFrom(v, arrList.DataType(), arrItems, start); err != nil {
			return err
		}
	}

	return nil
}
