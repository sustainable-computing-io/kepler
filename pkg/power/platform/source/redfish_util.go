/*
Copyright 2023.

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
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func getRedfishModel(access RedfishAccessInfo, endpoint string, model interface{}) error {
	username := access.Username
	password := access.Password
	host := access.Host

	// Create a HTTP client and set up the basic authentication header
	client := &http.Client{}
	url := host + endpoint
	req, err := http.NewRequest("GET", url, http.NoBody)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	req = req.WithContext(ctx)

	req.SetBasicAuth(username, password)

	// Send the request and check the response
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status: %v", resp.Status)
	}

	// Decode the response body into the provided model struct
	err = json.NewDecoder(resp.Body).Decode(model)
	if err != nil {
		return err
	}

	return nil
}

func getRedfishSystem(access RedfishAccessInfo) (*RedfishSystemModel, error) {
	var system RedfishSystemModel
	err := getRedfishModel(access, "/redfish/v1/Systems", &system)
	if err != nil {
		return nil, err
	}

	return &system, nil
}

func getRedfishPower(access RedfishAccessInfo, system string) (*RedfishPowerModel, error) {
	var power RedfishPowerModel
	err := getRedfishModel(access, "/redfish/v1/Chassis/"+system+"/Power%23/PowerControl", &power)
	if err != nil {
		return nil, err
	}

	return &power, nil
}
