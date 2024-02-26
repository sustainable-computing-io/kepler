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

package regressor

import (
	"math"
)

type LogisticPredictor struct {
	ModelWeights
}

func NewLogisticPredictor(weight ModelWeights) (predictor Predictor, err error) {
	if len(weight.AllWeights.CurveFitWeights) != 4 {
		return nil, ErrModelWeightsInvalid
	}
	return &LogisticPredictor{ModelWeights: weight}, nil
}

func (p *LogisticPredictor) predict(usageMetricNames []string, usageMetricValues [][]float64, systemMetaDataFeatureNames, systemMetaDataFeatureValues []string) []float64 {
	categoricalX, numericalX, _ := p.ModelWeights.getX(usageMetricNames, usageMetricValues, systemMetaDataFeatureNames, systemMetaDataFeatureValues)
	var basePower float64
	// TODO: update categoricalX transform (current no categorical value trained)
	for _, val := range categoricalX {
		basePower += val
	}
	var powers []float64
	A := p.ModelWeights.CurveFitWeights[0]
	x0 := p.ModelWeights.CurveFitWeights[1]
	k := p.ModelWeights.CurveFitWeights[2]
	off := p.ModelWeights.CurveFitWeights[3]
	for _, x := range numericalX {
		// note: curvefit use only index 0 feature
		power := basePower + A/(1+math.Exp(-k*(x[0]-x0))) + off
		powers = append(powers, power)
	}
	return powers
}
