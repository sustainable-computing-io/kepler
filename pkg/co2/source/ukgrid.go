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

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type UKGrid struct {
	collectionSupported bool
}

// Define a struct to hold the json data
type Intensity struct {
	Data []struct {
		From      string `json:"from"`
		To        string `json:"to"`
		Intensity struct {
			Forecast int    `json:"forecast"`
			Actual   int    `json:"actual"`
			Index    string `json:"index"`
		} `json:"intensity"`
	} `json:"data"`
}

func (d *UKGrid) Init() error {
	d.collectionSupported = false
	return nil
}

func (d *UKGrid) Shutdown() bool {
	return true
}

func (d *UKGrid) GetCarbonIntensity(url string) (int64, error) {
	client := &http.Client{}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Println(err)
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return 0, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return 0, err
	}

	var intensity Intensity
	err = json.Unmarshal(body, &intensity)
	if err != nil {
		fmt.Println(err)
		return 0, err
	}

	fmt.Printf("%+v\n", intensity)
	return int64(intensity.Data[0].Intensity.Actual), nil
}

func (d *UKGrid) IsCO2CollectionSupported() bool {
	return d.collectionSupported
}

func (d *UKGrid) SetCO2CollectionSupported(supported bool) {
	d.collectionSupported = supported
}
