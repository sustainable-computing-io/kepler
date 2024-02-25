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

// LparAPI defines an interface for issuing LPAR requests to ZHMC
//
//go:generate counterfeiter -o fakes/lpar.go --fake-name LparAPI . LparAPI
type LparAPI interface {
	CreateLPAR(cpcURI string, props *LparProperties) (string, int, *HmcError)
	ListLPARs(cpcURI string, query map[string]string) ([]LPAR, int, *HmcError)
	GetLparProperties(lparURI string) (*LparObjectProperties, int, *HmcError)
	UpdateLparProperties(lparURI string, props *LparProperties) (int, *HmcError)
	StartLPAR(lparURI string) (string, int, *HmcError)
	StopLPAR(lparURI string) (string, int, *HmcError)
	DeleteLPAR(lparURI string) (int, *HmcError)
	AttachStorageGroupToPartition(storageGroupURI string, request *StorageGroupPayload) (int, *HmcError)
	DetachStorageGroupToPartition(storageGroupURI string, request *StorageGroupPayload) (int, *HmcError)
	MountIsoImage(lparURI string, isoFile string, insFile string) (int, *HmcError)
	UnmountIsoImage(lparURI string) (int, *HmcError)

	ListNics(lparURI string) ([]string, int, *HmcError)
	FetchAsciiConsoleURI(lparURI string, request *AsciiConsoleURIPayload) (*AsciiConsoleURIResponse, int, *HmcError)

	GetEnergyDetailsforLPAR(lparURI string, props *EnergyRequestPayload) (uint64, int, *HmcError)

	AttachCryptoToPartition(lparURI string, request *CryptoConfig) (int, *HmcError)
}

type LparManager struct {
	client ClientAPI
}

type Wattage struct {
	Data      int `json:"data"`
	Timestamp int `json:"timestamp"`
}

type WattageData struct {
	Wattage []Wattage `json:"wattage"`
}

func NewLparManager(client ClientAPI) *LparManager {
	return &LparManager{
		client: client,
	}
}

/**
* GET /api/cpcs/{cpc-id}/partitions
* @cpcURI is the cpc object-uri
* @query is a key, value pairs array,
*        currently, supports 'name=$name_reg_expression'
*                            'status=PartitionStatus'
*                            'type=PartitionType'
* @return lpar array
* Return: 200 and LPARs array
*     or: 400, 404, 409
 */
func (m *LparManager) ListLPARs(cpcURI string, query map[string]string) ([]LPAR, int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, cpcURI, "/partitions")
	requestUrl = BuildUrlFromQuery(requestUrl, query)

	logger.Info(fmt.Sprintf("Request URL: %v, Method: %v", requestUrl, http.MethodGet))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodGet, requestUrl, nil, "")
	if err != nil {
		logger.Error("error on getting lpar's",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodGet),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return nil, status, err
	}

	if status == http.StatusOK {
		lpars := &LPARsArray{}
		err := json.Unmarshal(responseBody, lpars)
		if err != nil {
			logger.Error("error on unmarshalling adapters",
				zap.String("request url", fmt.Sprint(requestUrl)),
				zap.String("method", http.MethodGet),
				zap.Error(fmt.Errorf("%v", getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err))))
			return nil, status, getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err)
		}
		logger.Info(fmt.Sprintf("Response: get on lpars successfull, status: %v", status))
		return lpars.LPARS, status, nil
	}
	errorResponseBody := GenerateErrorFromResponse(responseBody)
	logger.Error("error listing lpar's",
		zap.String("request url", fmt.Sprint(requestUrl)),
		zap.String("method", http.MethodGet),
		zap.String("status: ", fmt.Sprint(status)),
		zap.Error(fmt.Errorf("%v", errorResponseBody)))
	return nil, status, errorResponseBody
}

/**
* GET /api/partitions/{partition-id}
* @lparURI is the object-uri
* Return: 200 and LparObjectProperties
*     or: 400, 404,
 */
func (m *LparManager) GetLparProperties(lparURI string) (*LparObjectProperties, int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, lparURI)

	logger.Info(fmt.Sprintf("Request URL: %v, Method: %v", requestUrl, http.MethodGet))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodGet, requestUrl, nil, "")
	if err != nil {
		logger.Error("error on getting lpar properties",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodGet),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return nil, status, err
	}

	if status == http.StatusOK {
		lparProps := LparObjectProperties{}
		err := json.Unmarshal(responseBody, &lparProps)
		if err != nil {
			return nil, status, getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err)
		}
		logger.Info(fmt.Sprintf("Response: request url: %v, method: %v, status: %v, property details, name: %v,description: %v"+
			"uri: %v, partition status: %v, partition type: %v, id: %v, processor mode: %v, ifl processors: %v, memory: %v",
			requestUrl, http.MethodGet, status, lparProps.Name, lparProps.Description, lparProps.URI, lparProps.Status, lparProps.Type,
			lparProps.ID, lparProps.ProcessorMode, lparProps.IflProcessors, lparProps.MaximumMemory))
		return &lparProps, status, nil
	}
	errorResponseBody := GenerateErrorFromResponse(responseBody)
	logger.Error("error getting lpar properties",
		zap.String("request url", fmt.Sprint(requestUrl)),
		zap.String("method", http.MethodGet),
		zap.String("status: ", fmt.Sprint(status)),
		zap.Error(fmt.Errorf("%v", errorResponseBody)))
	return nil, status, errorResponseBody
}

/**
* POST /api/cpcs/{cpc-id}/partitions
* @cpcURI is the cpc object-uri
* @props are LPAR properties'
* @return object-uri string
* Return: 200 and object-uri string
*     or: 400, 404, 409
 */
func (m *LparManager) CreateLPAR(cpcURI string, props *LparProperties) (string, int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, cpcURI, "/partitions")

	logger.Info(fmt.Sprintf("Request URL: %v, Method: %v", requestUrl, http.MethodPost))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodPost, requestUrl, props, "")
	if err != nil {
		logger.Error("error on getting lpar's",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodPost),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return "", status, err
	}

	if status == http.StatusCreated {
		responseObj := LparProperties{}
		err := json.Unmarshal(responseBody, &responseObj)
		if err != nil {
			return "", status, getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err)
		}
		if responseObj.URI != "" {
			logger.Info(fmt.Sprintf("Response: request url: %v, method: %v, status: %v, lpar uri: %v", requestUrl, http.MethodPost, status, responseObj.URI))
			return responseObj.URI, status, nil
		}
		logger.Error("error on starting lpar",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodPost),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(errors.New("empty job uri")))
		return "", status, getHmcErrorFromMsg(ERR_CODE_EMPTY_JOB_URI, ERR_MSG_EMPTY_JOB_URI)
	}
	errorResponseBody := GenerateErrorFromResponse(responseBody)
	logger.Error("error listing lpar's",
		zap.String("request url", fmt.Sprint(requestUrl)),
		zap.String("method", http.MethodPost),
		zap.String("status: ", fmt.Sprint(status)),
		zap.Error(fmt.Errorf("%v", errorResponseBody)))
	return "", status, errorResponseBody
}

/**
* POST /api/partitions/{partition-id}
* @lparURI is the object-uri
* Return: 204
*     or: 400, 403, 404, 409, 503,
 */
func (m *LparManager) UpdateLparProperties(lparURI string, props *LparProperties) (int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, lparURI)

	logger.Info(fmt.Sprintf("Request URL: %v, Method: %v", requestUrl, http.MethodPost))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodPost, requestUrl, props, "")
	if err != nil {
		logger.Error("error on getting lpar properties",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodPost),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return status, err
	}

	if status == http.StatusNoContent {
		logger.Info(fmt.Sprintf("update lpar properties completed, request url: %v, method: %v, status: %v", requestUrl, http.MethodPost, status))
		return status, nil
	}
	errorResponseBody := GenerateErrorFromResponse(responseBody)
	logger.Error("error updating lpar properties",
		zap.String("request url", fmt.Sprint(requestUrl)),
		zap.String("method", http.MethodPost),
		zap.String("status: ", fmt.Sprint(status)),
		zap.Error(fmt.Errorf("%v", errorResponseBody)))
	return status, errorResponseBody
}

/**
* POST /api/partitions/{partition-id}/operations/start
* @lparURI is the object-uri
* @return job-uri
* Return: 202 and job-uri
*     or: 400, 403, 404, 503,
 */
func (m *LparManager) StartLPAR(lparURI string) (string, int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, lparURI, "/operations/start")

	logger.Info(fmt.Sprintf("Request URL: %v, Method: %v", requestUrl, http.MethodPost))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodPost, requestUrl, nil, "")
	if err != nil {
		logger.Error("error on starting lpar",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodPost),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return "", status, err
	}

	if status == http.StatusAccepted {
		responseObj := StartStopLparResponse{}
		err := json.Unmarshal(responseBody, &responseObj)
		if err != nil {
			return "", status, getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err)
		}
		if responseObj.URI != "" {
			logger.Info(fmt.Sprintf("Response: request url: %v, method: %v, status: %v, lpar uri: %v", requestUrl, http.MethodPost, status, responseObj.URI))
			return responseObj.URI, status, nil
		}
		logger.Error("error on starting lpar",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodPost),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(errors.New("empty job uri")))
		return "", status, getHmcErrorFromMsg(ERR_CODE_EMPTY_JOB_URI, ERR_MSG_EMPTY_JOB_URI)
	}

	errorResponseBody := GenerateErrorFromResponse(responseBody)
	logger.Error("error starting lpar",
		zap.String("request url", fmt.Sprint(requestUrl)),
		zap.String("method", http.MethodPost),
		zap.String("status: ", fmt.Sprint(status)),
		zap.Error(fmt.Errorf("%v", errorResponseBody)))
	return "", status, errorResponseBody
}

/**
* POST /api/partitions/{partition-id}/operations/stop
* @lparURI is the object-uri
* @return job-uri
* Return: 202 and job-uri
*     or: 400, 403, 404, 503,
 */
func (m *LparManager) StopLPAR(lparURI string) (string, int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, lparURI, "/operations/stop")

	logger.Info(fmt.Sprintf("Request URL: %v, Method: %v", requestUrl, http.MethodPost))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodPost, requestUrl, nil, "")
	if err != nil {
		logger.Error("error on stopping lpar",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodPost),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return "", status, err
	}

	if status == http.StatusAccepted {
		responseObj := StartStopLparResponse{}
		err := json.Unmarshal(responseBody, &responseObj)
		if err != nil {
			logger.Error("error on unmarshalling adapters",
				zap.String("request url", fmt.Sprint(requestUrl)),
				zap.String("method", http.MethodGet),
				zap.Error(fmt.Errorf("%v", getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err))))
			return "", status, getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err)
		}
		if responseObj.URI != "" {
			logger.Info(fmt.Sprintf("Response: request url: %v, method: %v, status: %v, lpar uri: %v", requestUrl, http.MethodPost, status, responseObj.URI))
			return responseObj.URI, status, nil
		}
		logger.Error("error on stopping lpar",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodPost),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(errors.New("empty job uri")))
		return "", status, getHmcErrorFromMsg(ERR_CODE_EMPTY_JOB_URI, ERR_MSG_EMPTY_JOB_URI)
	}
	errorResponseBody := GenerateErrorFromResponse(responseBody)
	logger.Error("error stopping lpar",
		zap.String("request url", fmt.Sprint(requestUrl)),
		zap.String("method", http.MethodPost),
		zap.String("status: ", fmt.Sprint(status)),
		zap.Error(fmt.Errorf("%v", errorResponseBody)))
	return "", status, errorResponseBody
}

/**
* DELETE /api/partitions/{partition-id}
* @lparURI the lpar ID to be deleted
* Return: 204
*     or: 400, 403, 404, 409, 503
 */
func (m *LparManager) DeleteLPAR(lparURI string) (int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, lparURI)

	logger.Info(fmt.Sprintf("Request URL: %v, Method: %v", requestUrl, http.MethodDelete))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodDelete, requestUrl, nil, "")
	if err != nil {
		logger.Error("error deleting partition",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodDelete),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return status, err
	}

	if status == http.StatusNoContent {
		logger.Info(fmt.Sprintf("Response: partition deleted, request url: %v, method: %v, status: %v", requestUrl, http.MethodDelete, status))
		return status, nil
	}
	errorResponseBody := GenerateErrorFromResponse(responseBody)
	logger.Error("error deleting partition",
		zap.String("request url", fmt.Sprint(requestUrl)),
		zap.String("method", http.MethodDelete),
		zap.String("status: ", fmt.Sprint(status)),
		zap.Error(fmt.Errorf("%v", errorResponseBody)))
	return status, errorResponseBody
}

/**
* POST /api/partitions/{partition-id}/operations/mount-iso-image
* @lparURI is the object-uri
* Return: 204
*     or: 400, 403, 404, 409, 503
 */
func (m *LparManager) MountIsoImage(lparURI string, isoFile string, insFile string) (int, *HmcError) {
	pureIsoName := path.Base(isoFile)
	pureInsName := path.Base(insFile)
	query := map[string]string{
		"image-name":    pureIsoName,
		"ins-file-name": "/" + pureInsName,
	}
	imageData, byteErr := RetrieveBytes(isoFile)
	if byteErr != nil {
		logger.Error("error on retrieving iso file", zap.Error(byteErr))
	}
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, lparURI, "/operations/mount-iso-image")
	requestUrl = BuildUrlFromQuery(requestUrl, query)

	logger.Info(fmt.Sprintf("Request URL: %v, Method: %v, Parameters: iso file %v, insfile: %v", requestUrl, http.MethodPost, isoFile, insFile))

	status, responseBody, err := m.client.UploadRequest(http.MethodPost, requestUrl, imageData)
	if err != nil {
		logger.Error("error mounting iso image",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodPost),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return status, err
	}

	if status == http.StatusNoContent {
		logger.Info(fmt.Sprintf("Response: mounting iso image completed, request url: %v, method: %v, status: %v", requestUrl, http.MethodPost, status))
		return status, nil
	}
	errorResponseBody := GenerateErrorFromResponse(responseBody)
	logger.Error("error mounting iso image",
		zap.String("request url", fmt.Sprint(requestUrl)),
		zap.String("method", http.MethodPost),
		zap.String("status: ", fmt.Sprint(status)),
		zap.Error(fmt.Errorf("%v", errorResponseBody)))
	return status, errorResponseBody
}

/**
* POST /api/partitions/{partition-id}/operations/unmount-iso-image
* @lparURI is the object-uri
* Return: 204
*     or: 400, 403, 404, 409, 503
 */
func (m *LparManager) UnmountIsoImage(lparURI string) (int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, lparURI, "/operations/unmount-iso-image")

	logger.Info(fmt.Sprintf("Request URL: %v, Method: %v", requestUrl, http.MethodPost))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodPost, requestUrl, nil, "")
	if err != nil {
		logger.Error("error unmounting iso image",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodPost),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return status, err
	}

	if status == http.StatusNoContent {
		logger.Info(fmt.Sprintf("Response: iso image unmounted. request url: %v, method: %v, status: %v", requestUrl, http.MethodPost, status))
		return status, nil
	}

	errorResponseBody := GenerateErrorFromResponse(responseBody)
	logger.Error("error unmounting iso image",
		zap.String("request url", fmt.Sprint(requestUrl)),
		zap.String("method", http.MethodPost),
		zap.String("status: ", fmt.Sprint(status)),
		zap.Error(fmt.Errorf("%v", errorResponseBody)))
	return status, errorResponseBody
}

/**
* get_property('nic-uris') from LPAR
 */
func (m *LparManager) ListNics(lparURI string) ([]string, int, *HmcError) {
	props, status, err := m.GetLparProperties(lparURI)
	if err != nil {
		logger.Error("error listing nics",
			zap.String("request url", fmt.Sprint(lparURI)),
			zap.String("method", http.MethodGet),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return nil, status, err
	}
	logger.Info(fmt.Sprintf("request url: %v, method: %v, status: %v, nic uri's: %v", lparURI, status, http.MethodGet, props.NicUris))
	return props.NicUris, status, nil
}

// AttachStorageGroupToPartition

/**
* POST /api/partitions/{partition-id}/operations/attach-storage-group
* Return: 200
*     or: 400, 404, 409
 */
func (m *LparManager) AttachStorageGroupToPartition(lparURI string, request *StorageGroupPayload) (int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, lparURI, "/operations/attach-storage-group")

	logger.Info(fmt.Sprintf("Request URL: %v, Method: %v", requestUrl, http.MethodPost))
	logger.Info(fmt.Sprintf("request: %v", request))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodPost, requestUrl, request, "")

	if err != nil {
		logger.Error("error on attach storage group to partition",
			zap.String("request url", fmt.Sprint(lparURI)),
			zap.String("method", http.MethodPost),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return status, err
	}

	if status == http.StatusNoContent {
		logger.Info(fmt.Sprintf("Response: attach storage group to partition successfull, request url: %v, method: %v, status: %v", lparURI, http.MethodPost, status))
		return status, nil
	}

	errorResponseBody := GenerateErrorFromResponse(responseBody)
	logger.Error("error attaching storage group to partition",
		zap.String("request url", fmt.Sprint(lparURI)),
		zap.String("method", http.MethodPost),
		zap.String("status: ", fmt.Sprint(status)),
		zap.Error(fmt.Errorf("%v", errorResponseBody)))
	return status, errorResponseBody
}

// DetachStorageGroupToPartition
/**
* POST /api/partitions/{partition-id}/operations/detach-storage-group
* Return: 200
*     or: 400, 404, 409
 */
func (m *LparManager) DetachStorageGroupToPartition(lparURI string, request *StorageGroupPayload) (int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, lparURI, "/operations/detach-storage-group")

	logger.Info(fmt.Sprintf("Request URL: %v, Method: %v", requestUrl, http.MethodPost))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodPost, requestUrl, request, "")

	if err != nil {
		logger.Error("error on detach storage group to partition",
			zap.String("request url", fmt.Sprint(lparURI)),
			zap.String("method", http.MethodPost),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return status, err
	}

	if status == http.StatusNoContent {
		logger.Info(fmt.Sprintf("Response: detach storage group to partition successfull, request url: %v, method: %v, status: %v", lparURI, http.MethodPost, status))
		return status, nil
	}

	errorResponseBody := GenerateErrorFromResponse(responseBody)
	logger.Error("error detaching storage group to partition",
		zap.String("request url", fmt.Sprint(lparURI)),
		zap.String("method", http.MethodPost),
		zap.String("status: ", fmt.Sprint(status)),
		zap.Error(fmt.Errorf("%v", errorResponseBody)))
	return status, errorResponseBody
}

// FetchAsciiConsoleURI
/**
* POST /api/partitions/{partition-id}/operations/get-ascii-console-websocket-uri
* Return: 200 and ascii-console-websocket-uri and sessionID for the given lpar
*     or: 400, 404, 409
 */
func (m *LparManager) FetchAsciiConsoleURI(lparURI string, request *AsciiConsoleURIPayload) (*AsciiConsoleURIResponse, int, *HmcError) {
	// Start a new session for each ascii console URI
	consoleSessionID, status, err := m.client.LogonConsole()
	if err != nil {
		return nil, status, err
	}
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, lparURI, "/operations/get-ascii-console-websocket-uri")

	logger.Info(fmt.Sprintf("Request URL: %v, Method: %v", requestUrl, http.MethodPost))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodPost, requestUrl, request, consoleSessionID)

	if err != nil {
		logger.Error("error on fetch ascii console uri",
			zap.String("request url", fmt.Sprint(lparURI)),
			zap.String("method", http.MethodPost),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return nil, status, err
	}

	if status == http.StatusOK {
		responseObj := &AsciiConsoleURIResponse{}

		err := json.Unmarshal(responseBody, &responseObj)
		if err != nil {
			logger.Error("error on unmarshalling adapters",
				zap.String("request url", fmt.Sprint(requestUrl)),
				zap.String("method", http.MethodGet),
				zap.Error(fmt.Errorf("%v", getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err))))
			return nil, status, getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err)
		}
		if responseObj.URI != "" {
			newResponseObj := &AsciiConsoleURIResponse{
				URI:       path.Join(requestUrl.Host, responseObj.URI),
				SessionID: consoleSessionID,
			}

			logger.Info(fmt.Sprintf("Response: request url: %v, method: %v, status: %v, ascii console object: %v", lparURI, http.MethodPost, status, responseObj))
			return newResponseObj, status, nil
		}
		logger.Error("error on fetch ascii console uri",
			zap.String("request url", fmt.Sprint(lparURI)),
			zap.String("method", http.MethodPost),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(errors.New("empty job uri")))
		return responseObj, status, getHmcErrorFromMsg(ERR_CODE_EMPTY_JOB_URI, ERR_MSG_EMPTY_JOB_URI)
	}

	errorResponseBody := GenerateErrorFromResponse(responseBody)
	logger.Error("error on fetch ascii console uri",
		zap.String("request url", fmt.Sprint(lparURI)),
		zap.String("method", http.MethodPost),
		zap.String("status: ", fmt.Sprint(status)),
		zap.Error(fmt.Errorf("%v", errorResponseBody)))
	return nil, status, errorResponseBody
}

// GetEnergyDetailsforLPAR
/**
* POST https://{hmc_addr}:{port}/api/logical-partitions/{logical-partition-id}/operations/get-historical-sustainability-data
* Return: 200
*     or: 400, 404, 409
* sample response:
	{
		"wattage": [{
			"data": 53,
			"timestamp": 1680394193292
		}, {
			"data": 52,
			"timestamp": 1680408593302
		}]
	}

*/
func (m *LparManager) GetEnergyDetailsforLPAR(lparURI string, props *EnergyRequestPayload) (uint64, int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()

	requestUrl.Path = path.Join(requestUrl.Path, lparURI, "/operations", "/get-historical-sustainability-data")
	logger.Info("Request URL:" + string(requestUrl.Path) + " Method:" + http.MethodPost + " props" + fmt.Sprint(props))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodPost, requestUrl, props, "")

	if err != nil {
		logger.Error("error on getting lpar's energy",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodGet),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return 0, status, err
	}

	logger.Info("Response : " + string(responseBody))

	if status == http.StatusOK {
		var wd WattageData
		err := json.Unmarshal(responseBody, &wd)
		if err != nil {
			return 0, status, getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err)
		}
		logger.Info("Response: get on lpars successfully, status:" + fmt.Sprint(status))
		return uint64(wd.Wattage[len(wd.Wattage)-1].Data), status, nil
	}
	errorResponseBody := GenerateErrorFromResponse(responseBody)
	return 0, status, errorResponseBody
}

func (m *LparManager) AttachCryptoToPartition(lparURI string, request *CryptoConfig) (int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, lparURI, "/operations/increase-crypto-configuration")

	logger.Info(fmt.Sprintf("Request URL: %v, Method: %v", requestUrl, http.MethodPost))
	logger.Info(fmt.Sprintf("request: %v", request))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodPost, requestUrl, request, "")

	if err != nil {
		logger.Error("error on attach crypto adapters and domains to partition",
			zap.String("request url", fmt.Sprint(lparURI)),
			zap.String("method", http.MethodPost),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return status, err
	}

	if status == http.StatusNoContent {
		logger.Info(fmt.Sprintf("Response: attach crypto adapters and domains to partition successfull, request url: %v, method: %v, status: %v", lparURI, http.MethodPost, status))
		return status, nil
	}

	errorResponseBody := GenerateErrorFromResponse(responseBody)
	logger.Error("error attaching crypto adapters and domains to partition",
		zap.String("request url", fmt.Sprint(lparURI)),
		zap.String("method", http.MethodPost),
		zap.String("status: ", fmt.Sprint(status)),
		zap.Error(fmt.Errorf("%v", errorResponseBody)))
	return status, errorResponseBody
}
