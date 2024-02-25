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

// CpcAPI defines an interface for issuing CPC requests to ZHMC
//
//go:generate counterfeiter -o fakes/cpc.go --fake-name CpcAPI . CpcAPI
type CpcAPI interface {
	ListCPCs(query map[string]string) ([]CPC, int, *HmcError)
	GetCPCProperties(cpcURI string) (*CPCProperties, int, *HmcError)
}

type CpcManager struct {
	client ClientAPI
}

func NewCpcManager(client ClientAPI) *CpcManager {
	return &CpcManager{
		client: client,
	}
}

/**
* GET /api/cpcs
* @query is a key, value pairs, currently, supports 'name=$name_reg_expression'
* Return: 200 and CPCs array
*     or: 400
 */
func (m *CpcManager) ListCPCs(query map[string]string) ([]CPC, int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, "/api/cpcs")
	requestUrl = BuildUrlFromQuery(requestUrl, query)

	logger.Info(fmt.Sprintf("Request URL: %v, Method: %v", requestUrl, http.MethodGet))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodGet, requestUrl, nil, "")
	if err != nil {
		logger.Error("error on listing cpc's",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodGet),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return nil, status, err
	}
	logger.Info(fmt.Sprintf("Response: request url: %v, method: %v, status: %v", requestUrl, http.MethodGet, status))
	if status == http.StatusOK {
		cpcs := &CpcsArray{}
		err := json.Unmarshal(responseBody, &cpcs)
		if err != nil {
			logger.Error("error on unmarshalling adapters",
				zap.String("request url", fmt.Sprint(requestUrl)),
				zap.String("method", http.MethodGet),
				zap.Error(fmt.Errorf("%v", getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err))))
			return nil, status, getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err)
		}
		return cpcs.CPCS, status, nil
	}
	errorResponseBody := GenerateErrorFromResponse(responseBody)
	logger.Error("error on listing cpc's",
		zap.String("request url", fmt.Sprint(requestUrl)),
		zap.String("method", http.MethodGet),
		zap.String("status: ", fmt.Sprint(status)),
		zap.Error(errors.New(errorResponseBody.Message)))

	return nil, status, errorResponseBody
}

/**
* GET /api/cpcs/{cpc-id}
* @return cpc properties
* Return: 200 and Cpcs properties
*     or: 400, 404, 409
 */
func (m *CpcManager) GetCPCProperties(cpcURI string) (*CPCProperties, int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, cpcURI)

	logger.Info(fmt.Sprintf("Request URL: %v, Method: %v", requestUrl, http.MethodGet))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodGet, requestUrl, nil, "")
	if err != nil {
		logger.Error("error on getting cpc properties",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodGet),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return nil, status, err
	}

	if status == http.StatusOK {
		cpcProps := &CPCProperties{}
		err := json.Unmarshal(responseBody, cpcProps)
		if err != nil {
			logger.Error("error on unmarshalling cpcs",
				zap.String("request url", fmt.Sprint(requestUrl)),
				zap.String("method", http.MethodGet),
				zap.Error(fmt.Errorf("%v", getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err))))
			return nil, status, getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err)
		}
		logger.Info(fmt.Sprintf("Response: request url: %v, method: %v, status: %v, cpcs: %v", requestUrl, http.MethodGet, status, cpcProps))
		return cpcProps, status, nil
	}
	errorResponseBody := GenerateErrorFromResponse(responseBody)
	logger.Error("error getting cpc properties",
		zap.String("request url", fmt.Sprint(requestUrl)),
		zap.String("method", http.MethodGet),
		zap.String("status: ", fmt.Sprint(status)),
		zap.Error(fmt.Errorf("%v", errorResponseBody)))
	return nil, status, errorResponseBody
}
