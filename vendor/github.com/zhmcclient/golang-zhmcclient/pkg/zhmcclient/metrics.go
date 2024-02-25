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
	"net/http"
	"path"
	"fmt"

	"go.uber.org/zap"
)

// MetricsAPI defines an interface for issuing LPAR requests to ZHMC
type MetricsAPI interface {
	GetLiveEnergyDetailsforLPAR(lparURI string) (uint64, int, *HmcError)
}

type MetricsManager struct {
	client 			ClientAPI
}

func NewMetricsManager(client ClientAPI) *MetricsManager {
	return &MetricsManager{
		client: client,
	}
}

// CollectMetrics collects metrics based on the "metrics context" created before.
// The return value is a list of MetricObject objects, which may
// belong to different metrics context groups or CPCs.
func (m *MetricsManager) CollectMetrics() (metricsObjectList []MetricsObject, err *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
 	requestUrl.Path = path.Join(requestUrl.Path, m.client.GetMetricsContext().URI)
	_, resp, err:= m.client.ExecuteRequest(http.MethodGet, requestUrl, nil, "")
	metricsObjectList = extractMetricObjects(m.client.GetMetricsContext(), string(resp))
	return
}

func (m *MetricsManager) GetLiveEnergyDetailsforLPAR(lparURI string) (uint64, int, *HmcError) {
	metricObjects, err := m.CollectMetrics()

	if err != nil {
		logger.Error("error on getting lpar's energy",
			zap.String("request url", fmt.Sprint(lparURI)),
			zap.String("method", http.MethodPost),
			zap.Error(fmt.Errorf("%v", err)))
		return 0, 0, err
	}
	// Find the item with the specified MetricsGroupName and ResourceURI
	var powerConsumptionWatts uint64 = 0
	for _, obj := range metricObjects {
		if obj.MetricsGroupName == "logical-partition-usage" && obj.ResourceURI == lparURI {
			switch value := obj.Metrics["power-consumption-watts"].(type) {
			case uint64:
				powerConsumptionWatts = value
			case int:
				powerConsumptionWatts = uint64(value)
			default:
				// Handle the case where the value has an unsupported type
				logger.Info("Unsupported type for power-consumption-watts")
			}
			break
		}
	}
	logger.Info("powerConsumptionWatts:" + fmt.Sprint(powerConsumptionWatts))
	return powerConsumptionWatts, 0, nil
}
