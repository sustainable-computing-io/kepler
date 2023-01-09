/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/*
estimate.go
estimate (node/pod) component and total power by calling Kepler estimator sidecar when it is available.
*/

package sidecar

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/sustainable-computing-io/kepler/pkg/model/types"
	"k8s.io/klog/v2"
)

// PowerRequest defines a request to Kepler Estimator to get estimated powers
type PowerRequest struct {
	UsageMetrics   []string    `json:"metrics"`
	UsageValues    [][]float64 `json:"values"`
	OutputType     string      `json:"output_type"`
	SystemFeatures []string    `json:"system_features"`
	SystemValues   []string    `json:"system_values"`
	ModelName      string      `json:"model_name"`
	SelectFilter   string      `json:"filter"`
}

// TotalPowerResponse defines a response of a list of total powers from Kepler Estimator
type TotalPowerResponse struct {
	Powers  []float64 `json:"powers"`
	Message string    `json:"msg"`
}

// ComponentPowerResponse defines a response of a map of component powers from Kepler Estimator
type ComponentPowerResponse struct {
	Powers  map[string][]float64 `json:"powers"`
	Message string               `json:"msg"`
}

// EstimatorSidecarConnector defines power estimator with Kepler Estimator sidecar
type EstimatorSidecarConnector struct {
	Socket         string
	UsageMetrics   []string
	OutputType     types.ModelOutputType
	SystemFeatures []string
	ModelName      string
	SelectFilter   string
	valid          bool
	isComponent    bool
}

// Init returns valid if estimator is connected and has compatible power model
func (c *EstimatorSidecarConnector) Init(systemValues []string) bool {
	zeros := make([]float64, len(c.UsageMetrics))
	usageValues := [][]float64{zeros}
	c.isComponent = types.IsComponentType(c.OutputType)
	_, err := c.makeRequest(usageValues, systemValues)
	if err == nil {
		c.valid = true
	} else {
		c.valid = false
	}
	return c.valid
}

// makeRequest makes a request to Kepler Estimator Sidecar to apply archived model and get predicted powers
func (c *EstimatorSidecarConnector) makeRequest(usageValues [][]float64, systemValues []string) (interface{}, error) {
	powerRequest := PowerRequest{
		ModelName:      c.ModelName,
		UsageMetrics:   c.UsageMetrics,
		UsageValues:    usageValues,
		OutputType:     c.OutputType.String(),
		SystemFeatures: c.SystemFeatures,
		SystemValues:   systemValues,
		SelectFilter:   c.SelectFilter,
	}
	powerRequestJSON, err := json.Marshal(powerRequest)
	if err != nil {
		klog.V(4).Infof("marshal error: %v (%v)", err, powerRequest)
		return nil, err
	}

	conn, err := net.Dial("unix", c.Socket)
	if err != nil {
		klog.V(4).Infof("dial error: %v", err)
		return nil, err
	}
	defer conn.Close()

	_, err = conn.Write(powerRequestJSON)

	if err != nil {
		klog.V(4).Infof("estimator write error: %v", err)
		return nil, err
	}
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		klog.V(4).Infof("estimator read error: %v", err)
		return nil, err
	}
	var powers interface{}
	if c.isComponent {
		var powerResponse ComponentPowerResponse
		err = json.Unmarshal(buf[0:n], &powerResponse)
		powers = powerResponse.Powers
	} else {
		var powerResponse TotalPowerResponse
		err = json.Unmarshal(buf[0:n], &powerResponse)
		powers = powerResponse.Powers
	}
	if err != nil {
		klog.V(4).Infof("estimator unmarshal error: %v (%s)", err, string(buf[0:n]))
		return nil, err
	}
	return powers, nil
}

// GetTotalPower makes a request to Kepler Estimator Sidecar and returns a list of total powers
func (c *EstimatorSidecarConnector) GetTotalPower(usageValues [][]float64, systemValues []string) ([]float64, error) {
	if !c.valid {
		return []float64{}, fmt.Errorf("invalid power model call: %s", c.OutputType.String())
	}
	powers, err := c.makeRequest(usageValues, systemValues)
	if err != nil {
		return []float64{}, err
	}
	return powers.([]float64), err
}

// GetComponentPower makes a request to Kepler Estimator Sidecar and return a list of total powers
func (c *EstimatorSidecarConnector) GetComponentPower(usageValues [][]float64, systemValues []string) (map[string][]float64, error) {
	if !c.valid {
		return map[string][]float64{}, fmt.Errorf("invalid power model call: %s", c.OutputType.String())
	}
	powers, err := c.makeRequest(usageValues, systemValues)
	if err != nil {
		return map[string][]float64{}, err
	}
	return powers.(map[string][]float64), err
}
