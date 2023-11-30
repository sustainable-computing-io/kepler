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

	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/model/types"
	"github.com/sustainable-computing-io/kepler/pkg/model/utils"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/components/source"
	"k8s.io/klog/v2"
)

const MaxProcesss = 500 // 256 pods and 2 processes per pod

// PowerRequest defines a request to Kepler Estimator to get estimated powers
type PowerRequest struct {
	FloatFeatureNames           []string    `json:"metrics"`
	UsageValues                 [][]float64 `json:"values"`
	OutputType                  string      `json:"output_type"`
	EnergySource                string      `json:"source"`
	SystemMetaDataFeatureNames  []string    `json:"system_features"`
	SystemMetaDataFeatureValues []string    `json:"system_values"`
	TrainerName                 string      `json:"trainer_name"`
	SelectFilter                string      `json:"filter"`
}

// PlatformPowerResponse defines a response of a list of total powers from Kepler Estimator
type PlatformPowerResponse struct {
	Powers  []float64 `json:"powers"`
	Message string    `json:"msg"`
}

// ComponentPowerResponse defines a response of a map of component powers from Kepler Estimator
type ComponentPowerResponse struct {
	Powers  map[string][]float64 `json:"powers"`
	Message string               `json:"msg"`
}

// EstimatorSidecar defines power estimator with Kepler Estimator sidecar
type EstimatorSidecar struct {
	Socket       string
	OutputType   types.ModelOutputType
	EnergySource string
	TrainerName  string
	SelectFilter string

	FloatFeatureNames           []string
	SystemMetaDataFeatureNames  []string
	SystemMetaDataFeatureValues []string

	floatFeatureValues [][]float64 // metrics per process/process/pod/node
	// idle power is calculated with the minimal resource utilization, which means that the system is at rest
	// due to performance reasons, we keep a shadow copy of the floatFeatureValues with 1 values
	floatFeatureValuesForIdlePower [][]float64 // metrics per process/process/pod/node
	// xidx represents the instance slide window position, where an instance can be process/process/pod/node
	xidx int

	enabled bool
}

// Start returns nil if estimator is connected and has compatible power model
func (c *EstimatorSidecar) Start() error {
	zeros := make([]float64, len(c.FloatFeatureNames))
	usageValues := [][]float64{zeros}
	c.enabled = false
	_, err := c.makeRequest(usageValues, c.SystemMetaDataFeatureValues)
	if err == nil {
		c.enabled = true
		return nil
	}
	return err
}

// makeRequest makes a request to Kepler Estimator EstimatorSidecar to apply archived model and get predicted powers
func (c *EstimatorSidecar) makeRequest(usageValues [][]float64, systemValues []string) (interface{}, error) {
	powerRequest := PowerRequest{
		TrainerName:                 c.TrainerName,
		FloatFeatureNames:           c.FloatFeatureNames,
		UsageValues:                 usageValues,
		OutputType:                  c.OutputType.String(),
		EnergySource:                c.EnergySource,
		SystemMetaDataFeatureNames:  c.SystemMetaDataFeatureNames,
		SystemMetaDataFeatureValues: systemValues,
		SelectFilter:                c.SelectFilter,
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
	var powerResponse ComponentPowerResponse
	err = json.Unmarshal(buf[0:n], &powerResponse)
	powers = powerResponse.Powers
	if err != nil {
		klog.V(4).Infof("estimator unmarshal error: %v (%s)", err, string(buf[0:n]))
		return nil, err
	}
	return powers, nil
}

// GetPlatformPower makes a request to Kepler Estimator EstimatorSidecar and returns a list of total powers
func (c *EstimatorSidecar) GetPlatformPower(isIdlePower bool) ([]float64, error) {
	if !c.enabled {
		return []float64{}, fmt.Errorf("disabled power model call: %s", c.OutputType.String())
	}
	featuresValues := c.floatFeatureValues[0:c.xidx]
	if isIdlePower {
		featuresValues = c.floatFeatureValuesForIdlePower[0:c.xidx]
	}
	compPowers, err := c.makeRequest(featuresValues, c.SystemMetaDataFeatureValues)
	if err != nil {
		return []float64{}, err
	}
	power := compPowers.(map[string][]float64)
	if len(power) == 0 {
		return []float64{}, err
	}
	if powers, found := power[config.PLATFORM]; !found {
		return []float64{}, fmt.Errorf("not found %s in response %v", config.PLATFORM, power)
	} else {
		return powers, nil
	}
}

// GetComponentsPower makes a request to Kepler Estimator EstimatorSidecar and return a list of total powers
func (c *EstimatorSidecar) GetComponentsPower(isIdlePower bool) ([]source.NodeComponentsEnergy, error) {
	if !c.enabled {
		return []source.NodeComponentsEnergy{}, fmt.Errorf("disabled power model call: %s", c.OutputType.String())
	}
	featuresValues := c.floatFeatureValues[0:c.xidx]
	if isIdlePower {
		featuresValues = c.floatFeatureValuesForIdlePower[0:c.xidx]
	}
	compPowers, err := c.makeRequest(featuresValues, c.SystemMetaDataFeatureValues)
	if err != nil {
		return []source.NodeComponentsEnergy{}, err
	}
	power := compPowers.(map[string][]float64)
	num := 0 // number of processes
	for _, vals := range power {
		// the vals list has one entry of the predicted value for each process
		num = len(vals)
		break
	}
	nodeComponentsPower := make([]source.NodeComponentsEnergy, num)

	for index := 0; index < num; index++ {
		pkgPower := utils.GetComponentPower(power, config.PKG, index)
		corePower := utils.GetComponentPower(power, config.CORE, index)
		uncorePower := utils.GetComponentPower(power, config.UNCORE, index)
		dramPower := utils.GetComponentPower(power, config.DRAM, index)
		nodeComponentsPower[index] = utils.FillNodeComponentsPower(pkgPower, corePower, uncorePower, dramPower)
	}

	return nodeComponentsPower, err
}

// GetComponentsPower returns GPU Power in Watts associated to each each process/process/pod
func (c *EstimatorSidecar) GetGPUPower(isIdlePower bool) ([]float64, error) {
	return []float64{}, fmt.Errorf("current power model does not support GPUs")
}

func (c *EstimatorSidecar) addFloatFeatureValues(x []float64) {
	for i, feature := range x {
		// floatFeatureValues is a cyclic list, where we only append a new value if it is necessary.
		if c.xidx < len(c.floatFeatureValues) {
			if i < len(c.floatFeatureValues[c.xidx]) {
				c.floatFeatureValues[c.xidx][i] = feature
				// we don't need to add idle power since it is already set as 0
			} else {
				c.floatFeatureValues[c.xidx] = append(c.floatFeatureValues[c.xidx], feature)
				c.floatFeatureValuesForIdlePower[c.xidx] = append(c.floatFeatureValuesForIdlePower[c.xidx], 0)
			}
		} else {
			// add new process
			c.floatFeatureValues = append(c.floatFeatureValues, []float64{})
			c.floatFeatureValuesForIdlePower = append(c.floatFeatureValuesForIdlePower, []float64{})
			// add feature of new process
			c.floatFeatureValues[c.xidx] = append(c.floatFeatureValues[c.xidx], feature)
			c.floatFeatureValuesForIdlePower[c.xidx] = append(c.floatFeatureValuesForIdlePower[c.xidx], 0)
		}
	}
	c.xidx += 1 // mode pointer to next process
}

// AddProcessFeatureValues adds the the x for prediction, which are the explanatory variables (or the independent variable) of regression.
// EstimatorSidecar is trained off-line then we cannot Add training samples. We might implement it in the future.
// The EstimatorSidecar does not differentiate node or process power estimation, the difference will only be the amount of resource utilization
func (c *EstimatorSidecar) AddProcessFeatureValues(x []float64) {
	c.addFloatFeatureValues(x)
}

// AddNodeFeatureValues adds the the x for prediction, which is the variable used to calculate the ratio.
// EstimatorSidecar is not trained, then we cannot Add training samples, only samples for prediction.
// The EstimatorSidecar does not differentiate node or process power estimation, the difference will only be the amount of resource utilization
func (c *EstimatorSidecar) AddNodeFeatureValues(x []float64) {
	c.addFloatFeatureValues(x)
}

// AddDesiredOutValue adds the the y, which is the response variable (or the dependent variable) of regression.
// EstimatorSidecar is trained off-line then we do not add Y for trainning. We might implement it in the future.
func (c *EstimatorSidecar) AddDesiredOutValue(y float64) {
}

// ResetSampleIdx set the sample vector index to 0 to overwrite the old samples with new ones for trainning or prediction.
func (c *EstimatorSidecar) ResetSampleIdx() {
	c.xidx = 0
}

// Train triggers the regressiong fit after adding data points to create a new power model.
// EstimatorSidecar is trained in the Model Server then we cannot trigger the trainning. We might implement it in the future.
func (c *EstimatorSidecar) Train() error {
	return nil
}

// IsEnabled returns true if the power model was trained and is active
func (c *EstimatorSidecar) IsEnabled() bool {
	return c.enabled
}

// GetModelType returns the model type
func (c *EstimatorSidecar) GetModelType() types.ModelType {
	return types.EstimatorSidecar
}

// GetProcessFeatureNamesList returns the list of float features that the model was configured to use
// The EstimatorSidecar does not differentiate node or process power estimation, the difference will only be the amount of resource utilization
func (c *EstimatorSidecar) GetProcessFeatureNamesList() []string {
	return c.FloatFeatureNames
}

// GetNodeFeatureNamesList returns the list of float features that the model was configured to use
// The EstimatorSidecar does not differentiate node or process power estimation, the difference will only be the amount of resource utilization
func (c *EstimatorSidecar) GetNodeFeatureNamesList() []string {
	return c.FloatFeatureNames
}
