/*
 * Copyright The OpenTelemetry Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *        http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package arrow

import (
	"github.com/apache/arrow/go/v12/arrow"
	"github.com/apache/arrow/go/v12/arrow/array"
	"go.opentelemetry.io/collector/pdata/pcommon"

	schema "github.com/f5/otel-arrow-adapter/pkg/otel/common/schema"
	"github.com/f5/otel-arrow-adapter/pkg/otel/common/schema/builder"
	"github.com/f5/otel-arrow-adapter/pkg/otel/constants"
	"github.com/f5/otel-arrow-adapter/pkg/werror"
)

// ResourceDT is the Arrow Data Type describing a resource.
var (
	ResourceDT = arrow.StructOf([]arrow.Field{
		{
			Name:     constants.ID,
			Type:     arrow.PrimitiveTypes.Uint16,
			Metadata: schema.Metadata(schema.Optional, schema.DeltaEncoding),
		},
		{
			Name:     constants.SchemaUrl,
			Type:     arrow.BinaryTypes.String,
			Metadata: schema.Metadata(schema.Optional, schema.Dictionary8),
		},
		{
			Name:     constants.DroppedAttributesCount,
			Type:     arrow.PrimitiveTypes.Uint32,
			Metadata: schema.Metadata(schema.Optional),
		},
	}...)
)

// ResourceBuilder is an Arrow builder for resources.
type ResourceBuilder struct {
	released bool

	rBuilder *builder.RecordBuilderExt

	builder *builder.StructBuilder      // `resource` builder
	aib     *builder.Uint16DeltaBuilder // attributes id builder
	schb    *builder.StringBuilder      // `schema_url` builder
	dacb    *builder.Uint32Builder      // `dropped_attributes_count` field builder
}

// NewResourceBuilder creates a new resource builder with a given allocator.
func NewResourceBuilder(builder *builder.StructBuilder) *ResourceBuilder {
	return ResourceBuilderFrom(builder)
}

// ResourceBuilderFrom creates a new resource builder from an existing struct builder.
func ResourceBuilderFrom(builder *builder.StructBuilder) *ResourceBuilder {
	aib := builder.Uint16DeltaBuilder(constants.ID)
	// As the attributes are sorted before insertion, the delta between two
	// consecutive attributes ID should always be <=1.
	// We are enforcing this constraint to make sure that the delta encoding
	// will always be used efficiently.
	aib.SetMaxDelta(1)

	return &ResourceBuilder{
		released: false,
		builder:  builder,
		aib:      aib,
		schb:     builder.StringBuilder(constants.SchemaUrl),
		dacb:     builder.Uint32Builder(constants.DroppedAttributesCount),
	}
}

// Append appends a new resource to the builder.
func (b *ResourceBuilder) Append(resource *pcommon.Resource, attrsAccu *Attributes16Accumulator) error {
	if b.released {
		return werror.Wrap(ErrBuilderAlreadyReleased)
	}

	return b.builder.Append(resource, func() error {
		// ToDo Move to AppendWithID in a future PR
		ID, err := attrsAccu.Append(resource.Attributes())
		if err != nil {
			return werror.Wrap(err)
		}
		if ID >= 0 {
			b.aib.Append(uint16(ID))
		} else {
			// ID == -1 when the attributes are empty.
			b.aib.AppendNull()
		}
		b.schb.AppendNull()
		b.dacb.AppendNonZero(resource.DroppedAttributesCount())
		return nil
	})
}

func (b *ResourceBuilder) AppendWithID(attrsID int64, resource pcommon.Resource, schemaUrl string) error {
	if b.released {
		return werror.Wrap(ErrBuilderAlreadyReleased)
	}

	return b.builder.Append(resource, func() error {
		if attrsID >= 0 {
			b.aib.Append(uint16(attrsID))
		} else {
			// ID == -1 when the attributes are empty.
			b.aib.AppendNull()
		}
		b.schb.AppendNonEmpty(schemaUrl)
		b.dacb.AppendNonZero(resource.DroppedAttributesCount())
		return nil
	})
}

// Build builds the resource array struct.
//
// Once the array is no longer needed, Release() must be called to free the
// memory allocated by the array.
func (b *ResourceBuilder) Build() (*array.Struct, error) {
	if b.released {
		return nil, werror.Wrap(ErrBuilderAlreadyReleased)
	}

	defer b.Release()
	return b.builder.NewStructArray(), nil
}

// Release releases the memory allocated by the builder.
func (b *ResourceBuilder) Release() {
	if !b.released {
		b.builder.Release()

		b.released = true
	}
}
