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

package otlp

import (
	"errors"
)

var (
	ErrNotArraySparseUnion = errors.New("not an arrow array.SparseUnion")
	ErrNotArrayInt32       = errors.New("not an arrow array.Int32")
	ErrNotArrayUint64      = errors.New("not an arrow array.Uint64")
	ErrNotArrayFloat64     = errors.New("not an arrow array.Float64")
	ErrNotArrayList        = errors.New("not an arrow array.List")
	ErrNotArrayBoolean     = errors.New("not an arrow array.Boolean")
	ErrUnknownTypeCode     = errors.New("unknown type code")
)
