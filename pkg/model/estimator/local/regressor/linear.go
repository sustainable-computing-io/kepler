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
linear.go
estimate (node/pod) component and total power by linear regression approach when trained model weights are available.
The model weights can be obtained by Kepler Model Server or configured initial model URL.
*/

package regressor

type LinearPredictor struct {
	ModelWeights
}

// NewLinearPredictor creates a new LinearPredictor instance with the provided ModelWeights
func NewLinearPredictor(weight ModelWeights) (predictor Predictor, err error) {
	if len(weight.AllWeights.CurveFitWeights) == 0 {
		return &LinearPredictor{ModelWeights: weight}, nil
	}
	return nil, ErrModelWeightsInvalid
}

func (p *LinearPredictor) predict(usageMetricNames []string, usageMetricValues [][]float64, systemMetaDataFeatureNames, systemMetaDataFeatureValues []string) []float64 {
	categoricalX, numericalX, numericalWeights := p.ModelWeights.getX(usageMetricNames, usageMetricValues, systemMetaDataFeatureNames, systemMetaDataFeatureValues)
	basePower := p.ModelWeights.AllWeights.BiasWeight
	for _, val := range categoricalX {
		basePower += val
	}
	var powers []float64
	for _, x := range numericalX {
		power := basePower
		for i, coeff := range numericalWeights {
			if coeff.Weight == 0 {
				continue
			}
			power += coeff.Weight * x[i]
		}
		powers = append(powers, power)
	}
	return powers
}
