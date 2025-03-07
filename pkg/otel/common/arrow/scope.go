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

	acommon "github.com/f5/otel-arrow-adapter/pkg/otel/common/schema"
	"github.com/f5/otel-arrow-adapter/pkg/otel/common/schema/builder"
	"github.com/f5/otel-arrow-adapter/pkg/otel/constants"
	"github.com/f5/otel-arrow-adapter/pkg/werror"
)

// ScopeDT is the Arrow Data Type describing a scope.
var (
	ScopeDT = arrow.StructOf([]arrow.Field{
		{Name: constants.ID, Type: arrow.PrimitiveTypes.Uint16, Metadata: acommon.Metadata(acommon.Optional, acommon.DeltaEncoding)},
		{Name: constants.Name, Type: arrow.BinaryTypes.String, Metadata: acommon.Metadata(acommon.Optional, acommon.Dictionary8)},
		{Name: constants.Version, Type: arrow.BinaryTypes.String, Metadata: acommon.Metadata(acommon.Optional, acommon.Dictionary8)},
		{Name: constants.DroppedAttributesCount, Type: arrow.PrimitiveTypes.Uint32, Metadata: acommon.Metadata(acommon.Optional)},
	}...)
)

type ScopeBuilder struct {
	released bool
	builder  *builder.StructBuilder
	nb       *builder.StringBuilder      // Name builder
	vb       *builder.StringBuilder      // Version builder
	aib      *builder.Uint16DeltaBuilder // attributes id builder
	dacb     *builder.Uint32Builder      // Dropped attributes count builder
}

// NewScopeBuilder creates a new instrumentation scope array builder with a given allocator.
func NewScopeBuilder(builder *builder.StructBuilder) *ScopeBuilder {
	return ScopeBuilderFrom(builder)
}

// ScopeBuilderFrom creates a new instrumentation scope array builder from an existing struct builder.
func ScopeBuilderFrom(sb *builder.StructBuilder) *ScopeBuilder {
	aib := sb.Uint16DeltaBuilder(constants.ID)
	// As the attributes are sorted before insertion, the delta between two
	// consecutive attributes ID should always be <=1.
	// We are enforcing this constraint to make sure that the delta encoding
	// will always be used efficiently.
	aib.SetMaxDelta(1)
	return &ScopeBuilder{
		released: false,
		builder:  sb,
		nb:       sb.StringBuilder(constants.Name),
		vb:       sb.StringBuilder(constants.Version),
		aib:      aib,
		dacb:     sb.Uint32Builder(constants.DroppedAttributesCount),
	}
}

// Append appends a new instrumentation scope to the builder.
func (b *ScopeBuilder) Append(scope *pcommon.InstrumentationScope, attrsAccu *Attributes16Accumulator) error {
	if b.released {
		return werror.Wrap(ErrBuilderAlreadyReleased)
	}

	return b.builder.Append(scope, func() error {
		b.nb.AppendNonEmpty(scope.Name())
		b.vb.AppendNonEmpty(scope.Version())

		// ToDo Move to AppendWithID in a future PR
		ID, err := attrsAccu.Append(scope.Attributes())
		if err != nil {
			return werror.Wrap(err)
		}
		if ID >= 0 {
			b.aib.Append(uint16(ID))
		} else {
			// ID == -1 when the attributes are empty.
			b.aib.AppendNull()
		}

		b.dacb.AppendNonZero(scope.DroppedAttributesCount())
		return nil
	})
}

func (b *ScopeBuilder) AppendWithAttrsID(ID int64, scope pcommon.InstrumentationScope) error {
	if b.released {
		return werror.Wrap(ErrBuilderAlreadyReleased)
	}

	return b.builder.Append(scope, func() error {
		b.nb.AppendNonEmpty(scope.Name())
		b.vb.AppendNonEmpty(scope.Version())

		if ID >= 0 {
			b.aib.Append(uint16(ID))
		} else {
			// ID == -1 when the attributes are empty.
			b.aib.AppendNull()
		}

		b.dacb.AppendNonZero(scope.DroppedAttributesCount())
		return nil
	})
}

// Build builds the instrumentation scope array struct.
//
// Once the array is no longer needed, Release() must be called to free the
// memory allocated by the array.
func (b *ScopeBuilder) Build() (*array.Struct, error) {
	if b.released {
		return nil, werror.Wrap(ErrBuilderAlreadyReleased)
	}

	defer b.Release()
	return b.builder.NewStructArray(), nil
}

// Release releases the memory allocated by the builder.
func (b *ScopeBuilder) Release() {
	if !b.released {
		b.builder.Release()

		b.released = true
	}
}
