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

package co2

import (
	"fmt"

	co2_source "github.com/sustainable-computing-io/kepler/pkg/co2/source"
	"github.com/sustainable-computing-io/kepler/pkg/config"
)

var (
	co2Impl co2Interface = &co2_source.UKGrid{}
	errLib               = fmt.Errorf("could not start carbon intensity collector")
)

type co2Interface interface {
	// Init initizalize and start the CO2 metric collector
	Init() error
	// Shutdown stops the CO2 metric collector
	Shutdown() bool
	// GetCarbonIntensity returns a value of mg/joule CO2
	GetCarbonIntensity(string) (int64, error)
	// IsCO2CollectionSupported returns if it is possible to use this collector
	IsCO2CollectionSupported() bool
	// SetCO2CollectionSupported manually set if it is possible to use this collector. This is for testing purpose only.
	SetCO2CollectionSupported(bool)
}

func Init() error {
	return errLib
}

func Shutdown() bool {
	if co2Impl != nil && config.EnabledCO2 {
		return co2Impl.Shutdown()
	}
	return true
}

func GetCarbonIntensity(url string) (int64, error) {
	if co2Impl != nil && config.EnabledCO2 {
		return co2Impl.GetCarbonIntensity("https://api.carbonintensity.org.uk/intensity")
	}
	return 0, nil
}

func IsCO2CollectionSupported() bool {
	if co2Impl != nil && config.EnabledCO2 {
		return co2Impl.IsCO2CollectionSupported()
	}
	return false
}

func SetCO2CollectionSupported(supported bool) {
	if co2Impl != nil && config.EnabledCO2 {
		co2Impl.SetCO2CollectionSupported(supported)
	}
}
