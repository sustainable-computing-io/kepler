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
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sustainable-computing-io/kepler/pkg/config"
	"k8s.io/klog/v2"
)

func getRedfishModel(access RedfishAccessInfo, endpoint string, model interface{}) error {
	username := access.Username
	password := access.Password
	host := access.Host

	// Create a HTTP client and set up the basic authentication header
	transport := &http.Transport{}
	if config.GetRedfishSkipSSLVerify() {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	client := &http.Client{
		Transport: transport,
	}
	url := host + endpoint
	req, err := http.NewRequest("GET", url, http.NoBody)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	req = req.WithContext(ctx)

	// add additional header: 'OData-Version': '4.0'
	req.Header.Add("OData-Version", "4.0")
	// add accept header: 'application/json'
	req.Header.Add("Accept", "application/json")
	// set User-Agent header
	req.Header.Set("User-Agent", "kepler")
	// set keep-alive header
	req.Header.Set("Connection", "keep-alive")
	// set basic auth
	req.SetBasicAuth(username, password)

	// Send the request and check the response
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if _, err := io.Copy(io.Discard, resp.Body); err != nil {
			klog.V(0).Infof("Failed to discard response body: %v", err)
		}
		resp.Body.Close()
	}()

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status: %v", resp.Status)
	}

	dec := json.NewDecoder(resp.Body)
	dec.DisallowUnknownFields()

	var returnErr error
	// Decode the response body into the provided model struct
	decErr := dec.Decode(model)

	// process the error and only return significant ones
	if decErr != nil {
		if strings.HasPrefix(decErr.Error(), "json: unknown field ") {
			// ignore unknown field error
			fieldName := strings.TrimPrefix(decErr.Error(), "json: unknown field ")
			klog.V(6).Infof("Request body contains unknown field %s", fieldName)
		} else {
			returnErr = decErr
			klog.V(5).Infof("Failed to decode response: %v", decErr)
		}
	}

	return returnErr
}

func getRedfishSystem(access RedfishAccessInfo) (*RedfishSystemModel, error) {
	var system RedfishSystemModel
	err := getRedfishModel(access, "/redfish/v1/Systems", &system)
	if err != nil {
		klog.V(1).Infof("Failed to get system: %v", err)
		return nil, err
	}

	return &system, nil
}

func getRedfishPower(access RedfishAccessInfo, system string) (*RedfishPowerModel, error) {
	var power RedfishPowerModel
	err := getRedfishModel(access, "/redfish/v1/Chassis/"+system+"/Power#/PowerControl", &power)
	if err != nil {
		klog.V(1).Infof("Failed to get power: %v", err)
		return nil, err
	}

	return &power, nil
}
