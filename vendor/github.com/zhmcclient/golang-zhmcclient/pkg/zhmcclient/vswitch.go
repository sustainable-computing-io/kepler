// Copyright 2021-2023 IBM Corp. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package zhmcclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"

	"go.uber.org/zap"
)

// VirtualSwitchAPI defines an interface for issuing VirtualSwitch requests to ZHMC
//
//go:generate counterfeiter -o fakes/vswitch.go --fake-name VirtualSwitchAPI . VirtualSwitchAPI
type VirtualSwitchAPI interface {
	ListVirtualSwitches(cpcURI string, query map[string]string) ([]VirtualSwitch, int, *HmcError)
	GetVirtualSwitchProperties(vSwitchURI string) (*VirtualSwitchProperties, int, *HmcError)
}

type VirtualSwitchManager struct {
	client ClientAPI
}

func NewVirtualSwitchManager(client ClientAPI) *VirtualSwitchManager {
	return &VirtualSwitchManager{
		client: client,
	}
}

/**
 * GET /api/cpcs/{cpc-id}/virtual-switches
 * @cpcURI the URI of the CPC
 * @return adapter array
 * Return: 200 and VirtualSwitches array
 *     or: 400, 404, 409
 */
func (m *VirtualSwitchManager) ListVirtualSwitches(cpcURI string, query map[string]string) ([]VirtualSwitch, int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, cpcURI, "virtual-switches")
	requestUrl = BuildUrlFromQuery(requestUrl, query)

	logger.Info(fmt.Sprintf("Request URL: %v", requestUrl))
	logger.Info(fmt.Sprintf("Request Method: %v", http.MethodGet))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodGet, requestUrl, nil, "")
	if err != nil {
		logger.Error("error listing virtual switches",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodGet),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return nil, status, err
	}

	if status == http.StatusOK {
		virtualSwitches := &VirtualSwitchesArray{}
		err := json.Unmarshal(responseBody, virtualSwitches)
		if err != nil {
			logger.Error("error on unmarshalling adapters",
				zap.String("request url", fmt.Sprint(requestUrl)),
				zap.String("method", http.MethodGet),
				zap.Error(fmt.Errorf("%v", getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err))))
			return nil, status, getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err)
		}
		logger.Info(fmt.Sprintf("Response: request url: %v, method: %v, status: %v, virtual switches: %v", requestUrl, http.MethodGet, status, virtualSwitches.VIRTUALSWITCHES))
		return virtualSwitches.VIRTUALSWITCHES, status, nil
	}
	errorResponseBody := GenerateErrorFromResponse(responseBody)
	logger.Error("error on listing virtual switches",
		zap.String("request url", fmt.Sprint(requestUrl)),
		zap.String("method", http.MethodGet),
		zap.String("status: ", fmt.Sprint(status)),
		zap.Error(errors.New(errorResponseBody.Message)))
	return nil, status, errorResponseBody
}

/**
 * GET /api/virtual-switches/{vswitch-id}
 * @cpcURI the ID of the virtual switch
 * @return adapter array
 * Return: 200 and VirtualSwitchProperties
 *     or: 400, 404, 409
 */
func (m *VirtualSwitchManager) GetVirtualSwitchProperties(vSwitchURI string) (*VirtualSwitchProperties, int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, vSwitchURI)

	logger.Info(fmt.Sprintf("Request URL: %v, Method: %v", requestUrl, http.MethodGet))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodGet, requestUrl, nil, "")
	if err != nil {
		logger.Error("error getting virtual switch properties",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodGet),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return nil, status, err
	}

	if status == http.StatusOK {
		virtualSwitch := &VirtualSwitchProperties{}
		err := json.Unmarshal(responseBody, virtualSwitch)
		if err != nil {
			logger.Error("error on unmarshalling adapters",
				zap.String("request url", fmt.Sprint(requestUrl)),
				zap.String("method", http.MethodGet),
				zap.Error(fmt.Errorf("%v", getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err))))
			return nil, status, getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err)
		}
		logger.Info(fmt.Sprintf("Response: request url: %v, method: %v, status: %v, virtual switch properties: %v", requestUrl, http.MethodGet, status, virtualSwitch))
		return virtualSwitch, status, nil
	}
	errorResponseBody := GenerateErrorFromResponse(responseBody)
	logger.Error("error on getting switch properties",
		zap.String("request url", fmt.Sprint(requestUrl)),
		zap.String("method", http.MethodGet),
		zap.String("status: ", fmt.Sprint(status)),
		zap.Error(errors.New(errorResponseBody.Message)))
	return nil, status, errorResponseBody
}
