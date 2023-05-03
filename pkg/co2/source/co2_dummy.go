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

package source

type CO2Dummy struct {
	collectionSupported bool
}

func (d *CO2Dummy) Init() error {
	d.collectionSupported = false
	return nil
}

func (d *CO2Dummy) Shutdown() bool {
	return true
}

func (d *CO2Dummy) GetCarbonIntensity(url string) (int64, error) {
	return 0, nil
}

func (d *CO2Dummy) IsCO2CollectionSupported() bool {
	return d.collectionSupported
}

func (d *CO2Dummy) SetCO2CollectionSupported(supported bool) {
	d.collectionSupported = supported
}
