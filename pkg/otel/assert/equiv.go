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

package assert

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Equiv asserts that two arrays of json.Marshaler are equivalent. Metrics, logs, and traces requests implement
// json.Marshaler and are considered equivalent if they have the same set of vPaths. A vPath is a path to a value
// in a json object. For example the vPath "resource.attributes.service.name=myservice" refers to the value "myservice"
// in the json object {"resource":{"attributes":{"service":{"name":"myservice"}}}}.
//
// The structure of the expected and actual json objects does not need to be exactly the same. For example, the following
// json objects are considered equivalent:
// [{"resource":{"attributes":{"service":"myservice", "version":"1.0"}}}]
// [{"resource":{"attributes":{"service":"myservice"}}}, {"resource":{"attributes":{"version":"1.0"}}}]
//
// This concept of equivalence is useful for testing the conversion OTLP to/from OTLP Arrow as this conversion doesn't
// necessarily preserve the structure of the original OTLP entity. Resource spans or scope spans can be split or merged
// during the conversion if the semantic is preserved.
func Equiv(t *testing.T, expected []json.Marshaler, actual []json.Marshaler) {
	t.Helper()
	expectedVPaths, err := vPaths(expected)
	if err != nil {
		assert.FailNow(t, "Failed to convert expected traces to canonical representation", err)
	}
	actualVPaths, err := vPaths(actual)
	if err != nil {
		assert.FailNow(t, "Failed to convert actual traces to canonical representation", err)
	}

	missingExpectedVPaths := difference(expectedVPaths, actualVPaths)
	missingActualVPaths := difference(actualVPaths, expectedVPaths)

	if len(missingExpectedVPaths) > 0 {
		fmt.Printf("Missing expected vPaths:\n")
		for _, vPath := range missingExpectedVPaths {
			fmt.Printf("+ %s\n", vPath)
		}
	}
	if len(missingActualVPaths) > 0 {
		fmt.Printf("Unexpected vPaths:\n")
		for _, vPath := range missingActualVPaths {
			fmt.Printf("- %s\n", vPath)
		}
	}
	if len(missingExpectedVPaths) > 0 || len(missingActualVPaths) > 0 {
		assert.FailNow(t, "Traces are not equivalent")
	}
}

func EquivFromBytes(t *testing.T, expected []byte, actual []byte) {
	t.Helper()
	expectedVPaths, err := vPathsFromBytes(expected)
	if err != nil {
		assert.FailNow(t, "Failed to convert expected traces to canonical representation", err)
	}
	actualVPaths, err := vPathsFromBytes(actual)
	if err != nil {
		assert.FailNow(t, "Failed to convert actual traces to canonical representation", err)
	}

	missingExpectedVPaths := difference(expectedVPaths, actualVPaths)
	missingActualVPaths := difference(actualVPaths, expectedVPaths)

	if len(missingExpectedVPaths) > 0 {
		fmt.Printf("Missing expected vPaths:\n")
		for _, vPath := range missingExpectedVPaths {
			fmt.Printf("+ %s\n", vPath)
		}
	}
	if len(missingActualVPaths) > 0 {
		fmt.Printf("Unexpected vPaths:\n")
		for _, vPath := range missingActualVPaths {
			fmt.Printf("- %s\n", vPath)
		}
	}
	if len(missingExpectedVPaths) > 0 || len(missingActualVPaths) > 0 {
		assert.FailNow(t, "Traces are not equivalent")
	}
}

// NotEquiv asserts that two arrays of json.Marshaler are not equivalent. See Equiv for the definition of equivalence.
func NotEquiv(t *testing.T, expected []json.Marshaler, actual []json.Marshaler) {
	t.Helper()
	expectedVPaths, err := vPaths(expected)
	if err != nil {
		assert.FailNow(t, "Failed to convert expected traces to canonical representation", err)
	}
	actualVPaths, err := vPaths(actual)
	if err != nil {
		assert.FailNow(t, "Failed to convert actual traces to canonical representation", err)
	}

	missingExpectedVPaths := difference(expectedVPaths, actualVPaths)
	missingActualVPaths := difference(actualVPaths, expectedVPaths)

	if len(missingExpectedVPaths) == 0 && len(missingActualVPaths) == 0 {
		assert.FailNow(t, "Traces should not be equivalent")
	}
}

func difference(a, b []string) []string {
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x] = struct{}{}
	}
	var diff []string
	for _, x := range a {
		if _, found := mb[x]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}

func vPaths(marshaler []json.Marshaler) ([]string, error) {
	jsonTraces, err := jsonify(marshaler)
	if err != nil {
		return nil, err
	}
	vPathMap := make(map[string]bool)

	for i := 0; i < len(jsonTraces); i++ {
		exportAllVPaths(jsonTraces[i], "", vPathMap)
	}

	paths := make([]string, 0, len(vPathMap))
	for vPath := range vPathMap {
		paths = append(paths, vPath)
	}

	return paths, nil
}

func vPathsFromBytes(json []byte) ([]string, error) {
	jsonMap, err := jsonifyFromBytes(json)
	if err != nil {
		return nil, err
	}
	vPathMap := make(map[string]bool)

	exportAllVPaths(jsonMap, "", vPathMap)

	paths := make([]string, 0, len(vPathMap))
	for vPath := range vPathMap {
		paths = append(paths, vPath)
	}

	return paths, nil
}

func exportAllVPaths(traces map[string]interface{}, currentVPath string, vPaths map[string]bool) {
	for key, value := range traces {
		localVPath := key
		if currentVPath != "" {
			localVPath = currentVPath + "." + key
		}
		switch v := value.(type) {
		case []interface{}:
			for i := 0; i < len(v); i++ {
				// TODO: this is an approximation that is good enough for now, medium-term we should compute the index key based on a signature of the non-array fields.
				if vMap, ok := v[i].(map[string]interface{}); ok {
					arrayVPath := localVPath + "[_]"
					exportAllVPaths(vMap, arrayVPath, vPaths)
				} else {
					arrayVPath := fmt.Sprintf("%s[%d]=%s", localVPath, i, fmt.Sprint(v[i]))
					vPaths[arrayVPath] = true
				}
			}
		case []string:
			vPaths[localVPath+"="+strings.Join(v, ",")] = true
		case []int64:
			vPaths[localVPath+"="+strings.Join(strings.Fields(fmt.Sprint(v)), ",")] = true
		case []float64:
			vPaths[localVPath+"="+strings.Join(strings.Fields(fmt.Sprint(v)), ",")] = true
		case []bool:
			vPaths[localVPath+"="+strings.Join(strings.Fields(fmt.Sprint(v)), ",")] = true
		case map[string]interface{}:
			exportAllVPaths(v, localVPath, vPaths)
		case string:
			vPaths[localVPath+"="+v] = true
		case int64:
			vPaths[localVPath+"="+fmt.Sprintf("%d", v)] = true
		case float64:
			vPaths[localVPath+"="+fmt.Sprintf("%f", v)] = true
		case bool:
			vPaths[localVPath+"="+fmt.Sprintf("%f", 123.456)] = true
		}
	}
}

func jsonify(marshaler []json.Marshaler) ([]map[string]interface{}, error) {
	jsonTraces := make([]map[string]interface{}, 0, len(marshaler))

	for i := 0; i < len(marshaler); i++ {
		jsonBytes, err := marshaler[i].MarshalJSON()
		if err != nil {
			return nil, err
		}
		jsonMap, err := jsonifyFromBytes(jsonBytes)
		if err != nil {
			return nil, err
		}
		jsonTraces = append(jsonTraces, jsonMap)
	}
	return jsonTraces, nil
}

func jsonifyFromBytes(jsonBytes []byte) (map[string]interface{}, error) {
	var jsonMap map[string]interface{}
	err := json.Unmarshal(jsonBytes, &jsonMap)
	if err != nil {
		return nil, err
	}
	return jsonMap, nil
}

// JSONCanonicalEq compares two JSON objects for equality after converting
// them to a canonical form. This is useful for comparing JSON objects that may
// have different key orders or array orders.
func JSONCanonicalEq(t *testing.T, expected interface{}, actual interface{}) {
	t.Helper()

	expected, err := jsonFrom(expected)
	require.NoError(t, err)
	actual, err = jsonFrom(actual)
	require.NoError(t, err)

	expectedID := CanonicalObjectID(expected)
	actualID := CanonicalObjectID(actual)

	assert.Equal(t, expectedID, actualID)
}

// CanonicalObjectID computes a unique ID for an object.
func CanonicalObjectID(object interface{}) string {
	if object == nil {
		return "null"
	}

	switch obj := object.(type) {
	case map[string]interface{}:
		return CanonicalMapID(obj)
	case []interface{}:
		return CanonicalSliceID(obj)
	case []map[string]interface{}:
		return CanonicalSliceMapID(obj)
	case int64:
		return strconv.FormatInt(obj, 10)
	case float64:
		return strconv.FormatFloat(obj, 'f', -1, 64)
	case string:
		return fmt.Sprintf("%q", obj)
	case bool:
		return strconv.FormatBool(obj)
	default:
		fmt.Printf("canonical id: unknown type (object: %v)\n", object)
		return fmt.Sprintf("%v", object)
	}
}

// CanonicalMapID computes a unique ID for a map.
// Sort the keys to ensure a consistent order.
func CanonicalMapID(object map[string]interface{}) string {
	var keys []string
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var ID strings.Builder
	ID.WriteString("{")
	for _, key := range keys {
		if ID.Len() > 1 {
			ID.WriteString(",")
		}
		ID.WriteString("\"")
		ID.WriteString(key)
		ID.WriteString("\":")
		ID.WriteString(CanonicalObjectID(object[key]))
	}
	ID.WriteString("}")
	return ID.String()
}

// CanonicalSliceID computes a unique ID for a slice.
func CanonicalSliceID(slice []interface{}) string {
	var itemIDs []string

	for _, item := range slice {
		itemIDs = append(itemIDs, CanonicalObjectID(item))
	}
	sort.Strings(itemIDs)

	var ID strings.Builder
	ID.WriteString("[")
	for i, itemID := range itemIDs {
		if i > 0 {
			ID.WriteString(",")
		}
		ID.WriteString(itemID)
	}
	ID.WriteString("]")

	return ID.String()
}

// CanonicalSliceMapID computes a unique ID for a slice of maps.
func CanonicalSliceMapID(slice []map[string]interface{}) string {
	var itemIDs []string

	for _, item := range slice {
		itemIDs = append(itemIDs, CanonicalMapID(item))
	}
	sort.Strings(itemIDs)

	var ID strings.Builder
	ID.WriteString("[")
	for i, itemID := range itemIDs {
		if i > 0 {
			ID.WriteString(",")
		}
		ID.WriteString(itemID)
	}
	ID.WriteString("]")

	return ID.String()
}

// jsonFrom converts a string or a byte slice to a Go object representing
// this JSON object.
func jsonFrom(value interface{}) (interface{}, error) {
	switch v := value.(type) {
	case string:
		return jsonFromBytes([]byte(v))
	case []byte:
		return jsonFromBytes(v)
	case []json.Marshaler:
		return jsonify(v)
	default:
		return nil, fmt.Errorf("unsupported type: %T", value)
	}
}

// jsonFromBytes converts a byte slice, representing a JSON object, to a Go
// map or a slice of Go map.
func jsonFromBytes(jsonBytes []byte) (interface{}, error) {
	var jsonMap map[string]interface{}
	err := json.Unmarshal(jsonBytes, &jsonMap)
	if err != nil {
		var jsonArray []interface{}
		err = json.Unmarshal(jsonBytes, &jsonArray)
		if err != nil {
			return nil, err
		}
		return jsonArray, nil
	}
	return jsonMap, nil
}
