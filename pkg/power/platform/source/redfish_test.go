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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRedFishClient_IsPowerSupported(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redfish/v1/Systems" {
			system := RedfishSystemModel{
				Name: "Test System",
				Members: []struct {
					OdataID string `json:"@odata.id"`
				}{
					{
						OdataID: "/redfish/v1/Chassis/1",
					},
					{
						OdataID: "/redfish/v1/Chassis/2",
					},
				},
			}
			if err := json.NewEncoder(w).Encode(system); err != nil {
				fmt.Println(err)
			}
		} else if r.URL.Path == "/redfish/v1/Chassis/1/Power" || r.URL.Path == "/redfish/v1/Chassis/2/Power" {
			power := RedfishPowerModel{
				Name: "Test Power",
				PowerControl: []PowerControl{
					{
						PowerConsumedWatts: 100,
					},
				},
			}
			if err := json.NewEncoder(w).Encode(power); err != nil {
				fmt.Println(err)
			}
		} else {
			fmt.Printf("Path not found: %s\n", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	defer server.Close()
	fmt.Println("Mock server listening on", server.URL)
	// Configure the access details for the mock server
	access := RedfishAccessInfo{
		Username: "testuser",
		Password: "testpass",
		Host:     server.URL,
	}

	// Create a new Redfish client
	client := &RedFishClient{
		accessInfo:    access,
		systems:       []*RedfishSystemPowerResult{},
		probeInterval: 30,
	}

	// Check if power is supported
	isPowerSupported := client.IsSystemCollectionSupported()
	if !isPowerSupported {
		t.Error("Expected power support, but got false")
	}
	// stop the client
	client.StopPower()
}
