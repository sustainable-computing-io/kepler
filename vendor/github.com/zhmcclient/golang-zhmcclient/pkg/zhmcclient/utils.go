// Copyright 2021-2023 IBM Corp. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package zhmcclient

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type HmcErrorCode int

const (
	ERR_CODE_HMC_INVALID_URL HmcErrorCode = iota + 1000
	ERR_CODE_HMC_BAD_REQUEST
	ERR_CODE_HMC_EMPTY_RESPONSE
	ERR_CODE_HMC_READ_RESPONSE_FAIL
	ERR_CODE_HMC_TRACE_REQUEST_FAIL
	ERR_CODE_HMC_CREATE_METRICS_CTX_FAIL
	ERR_CODE_HMC_EXECUTE_FAIL
	ERR_CODE_HMC_MARSHAL_FAIL
	ERR_CODE_HMC_UNMARSHAL_FAIL
	ERR_CODE_EMPTY_JOB_URI
)

const (
	ERR_MSG_INSECURE_URL   = "https is used for the client for secure reason."
	ERR_MSG_EMPTY_RESPONSE = "http response is empty."
	ERR_MSG_EMPTY_JOB_URI  = "empty job-uri."
)

type HmcError struct {
	Reason  int    `json:"reason"`
	Message string `json:"message"`
}

// Metric group names, which are required when creating metrics context.
// For details, refer to "IBM Z Hardware Management Console Web Services API".
const (
	ZCpcEnvironmentalsAndPower = "zcpc-environmentals-and-power"
	EnvironmentalPowerStatus   = "environmental-power-status"
)

type metricDef struct {
	Name string `json:"metric-name"` // metric field name
	Type string `json:"metric-type"` // golang type of this metric value
}

type metricGroupDef struct {
	MetricsGroupName string      `json:"group-name"`
	MetricDefs       []metricDef `json:"metric-infos"`
}

// MetricsContextDef represents a "Metrics Context" resource, which is
// associated with any following metrics collection.
type MetricsContextDef struct {
	URI                   string           `json:"metrics-context-uri"`
	MetricGroupDefs       []metricGroupDef `json:"metric-group-infos"`
	MetricGroupDefsByName map[string]metricGroupDef
}

// MetricsObject represents the metric values of a metrics group
// for a single resource at a single point in time.
type MetricsObject struct {
	MetricsGroupName string
	TimeStamp        string
	ResourceURI      string
	Metrics          map[string]interface{}
}

func (e HmcError) Error() string {
	return fmt.Sprintf("HmcError - Reason: %d, %s", e.Reason, e.Message)
}

func getHmcErrorFromErr(reason HmcErrorCode, err error) *HmcError {
	return &HmcError{
		Reason:  int(reason),
		Message: err.Error(),
	}
}

func getHmcErrorFromMsg(reason HmcErrorCode, err string) *HmcError {
	return &HmcError{
		Reason:  (int)(reason),
		Message: err,
	}
}

func BuildUrlFromQuery(url *url.URL, query map[string]string) *url.URL {
	if query != nil {
		q := url.Query()
		for key, value := range query {
			q.Add(key, value)
		}
		url.RawQuery = q.Encode()
	}
	return url
}

func RetrieveBytes(filename string) ([]byte, error) {
	file, err := os.Open(filename)

	if err != nil {
		return nil, err
	}
	defer file.Close()

	stats, statsErr := file.Stat()
	if statsErr != nil {
		return nil, statsErr
	}

	var size int64 = stats.Size()
	bytes := make([]byte, size)
	bufr := bufio.NewReader(file)
	_, err = bufr.Read(bytes)
	return bytes, err
}

func GenerateErrorFromResponse(responseBodyStream []byte) *HmcError {
	errBody := &HmcError{}
	err := json.Unmarshal(responseBodyStream, errBody)

	if err != nil {
		return &HmcError{
			Reason:  -1,
			Message: "Unknown error.",
		}

	}

	return errBody
}

func newMetricsContext(properties []byte) (*MetricsContextDef, error) {
	mc := new(MetricsContextDef)

	if err := json.Unmarshal(properties, mc); err != nil {
		return nil, err
	}

	mc.MetricGroupDefsByName = make(map[string]metricGroupDef)
	for _, mgDef := range mc.MetricGroupDefs {
		mc.MetricGroupDefsByName[mgDef.MetricsGroupName] = mgDef
	}

	return mc, nil
}

// extract MetricGroup objects from response of a metrics retrieve request
func extractMetricObjects(mc *MetricsContextDef, metricsStr string) []MetricsObject {
	var moList []MetricsObject
	var mDefs []metricDef

	metricGroupName := ""
	resourceURL := ""
	timeStamp := ""

	state := 0
	lines := strings.Split(metricsStr, "\n")

	for _, line := range lines {

		switch state {
		case 0:
			{
				// start or just finish processing a metrics group
				if line == "" {
					// Skip initial(or trailing) empty lines
					continue
				} else {
					// Process the next metrics group
					metricGroupName = strings.Trim(line, `"`)
					mDefs = mc.MetricGroupDefsByName[metricGroupName].MetricDefs
					state = 1
				}
			}
		case 1:
			{
				if line == "" {
					//No(or no more) MetricObject items in this metrics group.
					state = 0
				} else {
					// There are MetricsObject items
					resourceURL = strings.Trim(line, `"`)
					state = 2
				}
			}
		case 2:
			{
				// Process the timestamp
				timeStamp = line
				state = 3
			}
		case 3:
			{
				if line != "" {
					// Process the metric values in the ValueRow line
					mValues := strings.Split(line, `,`)
					metrics := make(map[string]interface{})
					for Index, mDef := range mDefs {
						metrics[mDef.Name] = metricValueConvert(mValues[Index], mDef.Type)
					}
					mo := MetricsObject{
						MetricsGroupName: metricGroupName,
						TimeStamp:        timeStamp,
						ResourceURI:      resourceURL,
						Metrics:          metrics,
					}
					moList = append(moList, mo)
					// stay in this state, for more ValueRow lines
				} else {
					// On the empty line after the last ValueRow line
					state = 1
				}
			}
		}
	}

	return moList
}

// get metric value from a metric value strings
func metricValueConvert(valueStr string, metricType string) interface{} {
	if metricType == "bool" {
		lowerStr := strings.ToLower(valueStr)
		return lowerStr

	} else if metricType == "string" {
		return strings.Trim(valueStr, `"`)
	}

	switch metricType {
	case "boolean-metric":
		returnStr, _ := strconv.ParseBool(valueStr)
		return returnStr
	case "byte-metric", "short-metric", "integer-metric":
		returnStr, _ := strconv.Atoi(valueStr)
		return returnStr
	case "long-metric":
		returnStr, _ := strconv.ParseInt(valueStr, 10, 64)
		return returnStr
	case "string-metric":
		return valueStr
	case "double-metric":
		returnStr, _ := strconv.ParseFloat(valueStr, 64)
		return returnStr
	}

	return ""
}
