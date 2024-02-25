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

// NicAPI defines an interface for issuing NIC requests to ZHMC
//
//go:generate counterfeiter -o fakes/nic.go --fake-name NicAPI . NicAPI
type NicAPI interface {
	CreateNic(lparURI string, nic *NIC) (string, int, *HmcError)
	DeleteNic(nicURI string) (int, *HmcError)
	GetNicProperties(nicURI string) (*NIC, int, *HmcError)
	UpdateNicProperties(nicURI string, props *NIC) (int, *HmcError)
}

type NicManager struct {
	client ClientAPI
}

func NewNicManager(client ClientAPI) *NicManager {
	return &NicManager{
		client: client,
	}
}

/**
* POST /api/partitions/{partition-id}/nics
* @ return element-uri
* Return: 201 and element-uri
*     or: 400, 403, 404, 409, 503,
 */
func (m *NicManager) CreateNic(lparURI string, nic *NIC) (string, int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, lparURI, "nics")

	logger.Info(fmt.Sprintf("Request URL: %v, Method: %v", requestUrl, http.MethodPost))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodPost, requestUrl, nic, "")
	if err != nil {
		logger.Error("error on create nic",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodPost),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return "", status, err
	}

	if status == http.StatusCreated {
		uriObj := NicCreateResponse{}
		err := json.Unmarshal(responseBody, &uriObj)
		if err != nil {
			logger.Error("error on unmarshalling adapters",
				zap.String("request url", fmt.Sprint(requestUrl)),
				zap.String("method", http.MethodPost),
				zap.Error(fmt.Errorf("%v", getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err))))
			return "", status, getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err)
		}
		logger.Info(fmt.Sprintf("Response: nic created, request url: %v, method: %v, status: %v, nic uri: %v", requestUrl, http.MethodPost, status, uriObj.URI))
		return uriObj.URI, status, nil
	}
	errorResponseBody := GenerateErrorFromResponse(responseBody)
	logger.Error("error on create nic",
		zap.String("request url", fmt.Sprint(requestUrl)),
		zap.String("method", http.MethodPost),
		zap.String("status: ", fmt.Sprint(status)),
		zap.Error(errors.New(errorResponseBody.Message)))
	return "", status, errorResponseBody

}

/**
* DELETE /api/partitions/{partition-id}/nics/{nic-id}
* Return: 204
*     or: 400, 403, 404, 409, 503
 */
func (m *NicManager) DeleteNic(nicURI string) (int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, nicURI)

	logger.Info(fmt.Sprintf("Request URL: %v, Method: %v", requestUrl, http.MethodDelete))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodDelete, requestUrl, nil, "")
	if err != nil {
		logger.Error("error on delete nic",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodDelete),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return status, err
	}

	if status == http.StatusNoContent {
		logger.Info(fmt.Sprintf("Response: nic deleted, request url: %v, method: %v, status: %v", requestUrl, http.MethodDelete, status))
		return status, nil
	}

	errorResponseBody := GenerateErrorFromResponse(responseBody)
	logger.Error("error on delete nic",
		zap.String("request url", fmt.Sprint(requestUrl)),
		zap.String("method", http.MethodDelete),
		zap.String("status: ", fmt.Sprint(status)),
		zap.Error(fmt.Errorf("%v", errorResponseBody)))
	return status, errorResponseBody
}

/**
* GET /api/partitions/{partition-id}/nics/{nic-id}
* Return: 200 and NIC
*     or: 400, 404,
 */
func (m *NicManager) GetNicProperties(nicURI string) (*NIC, int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, nicURI)

	logger.Info(fmt.Sprintf("Request URL: %v, Method: %v", requestUrl, http.MethodGet))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodGet, requestUrl, nil, "")
	if err != nil {
		logger.Error("error on get nic properties",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodGet),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return nil, status, err
	}

	if status == http.StatusOK {
		nic := NIC{}
		err := json.Unmarshal(responseBody, &nic)
		if err != nil {
			logger.Error("error on unmarshalling adapters",
				zap.String("request url", fmt.Sprint(requestUrl)),
				zap.String("method", http.MethodGet),
				zap.Error(fmt.Errorf("%v", getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err))))
			return nil, status, getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err)
		}
		logger.Info(fmt.Sprintf("Response: request url: %v, method: %v, status: %v, nic propeties: %v", requestUrl, http.MethodGet, status, &nic))
		return &nic, status, nil
	}
	errorResponseBody := GenerateErrorFromResponse(responseBody)
	logger.Error("error on get nic properties",
		zap.String("request url", fmt.Sprint(requestUrl)),
		zap.String("method", http.MethodGet),
		zap.String("status: ", fmt.Sprint(status)),
		zap.Error(fmt.Errorf("%v", errorResponseBody)))
	return nil, status, errorResponseBody
}

/**
* POST /api/partitions/{partition-id}/nics/{nic-id}
* Return: 204
*     or: 400, 404,
 */
func (m *NicManager) UpdateNicProperties(nicURI string, props *NIC) (int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, nicURI)

	logger.Info(fmt.Sprintf("Request URL: %v, Method: %v", requestUrl, http.MethodPost))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodPost, requestUrl, props, "")
	if err != nil {
		logger.Error("error on update nic properties",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodPost),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return status, err
	}

	if status == http.StatusNoContent {
		logger.Info(fmt.Sprintf("Response: request url: %v, method: %v, status: %v", requestUrl, http.MethodGet, status))
		return status, nil
	}
	errorResponseBody := GenerateErrorFromResponse(responseBody)
	logger.Error("error on update nic properties",
		zap.String("request url", fmt.Sprint(requestUrl)),
		zap.String("method", http.MethodGet),
		zap.String("status: ", fmt.Sprint(status)),
		zap.Error(fmt.Errorf("%v", errorResponseBody)))
	return status, errorResponseBody
}
