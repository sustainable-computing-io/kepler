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
	"fmt"
	"net/http"
	"path"

	"go.uber.org/zap"
)

// StorageGroupAPI defines an interface for issuing NIC requests to ZHMC
//go:generate counterfeiter -o fakes/sgroup.go --fake-name StorageGroupAPI . StorageGroupAPI

type StorageGroupAPI interface {
	ListStorageGroups(storageGroupURI string, cpcUri string) ([]StorageGroup, int, *HmcError)
	GetStorageGroupProperties(storageGroupURI string) (*StorageGroupProperties, int, *HmcError)
	ListStorageVolumes(storageGroupURI string) ([]StorageVolume, int, *HmcError)
	GetStorageVolumeProperties(storageVolumeURI string) (*StorageVolume, int, *HmcError)
	UpdateStorageGroupProperties(storageGroupURI string, updateRequest *StorageGroupProperties) (int, *HmcError)
	FulfillStorageGroup(storageGroupURI string, updateRequest *StorageGroupProperties) (int, *HmcError)
	CreateStorageGroups(storageGroupURI string, storageGroup *CreateStorageGroupProperties) (*StorageGroupCreateResponse, int, *HmcError)
	GetStorageGroupPartitions(storageGroupURI string, query map[string]string) (*StorageGroupPartitions, int, *HmcError)
	DeleteStorageGroup(storageGroupURI string) (int, *HmcError)
}

type StorageGroupManager struct {
	client ClientAPI
}

func NewStorageGroupManager(client ClientAPI) *StorageGroupManager {
	return &StorageGroupManager{
		client: client,
	}
}

/**
 * GET /api/storage-groups
 * @cpcURI the URI of the CPC
 * @return storage group array
 * Return: 200 and Storage Group array
 *     or: 400, 404, 409
 */
func (m *StorageGroupManager) ListStorageGroups(storageGroupURI string, cpcUri string) ([]StorageGroup, int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, storageGroupURI)
	query := map[string]string{
		"cpc-uri": cpcUri,
	}
	requestUrl = BuildUrlFromQuery(requestUrl, query)

	logger.Info(fmt.Sprintf("Request URL: %v, Method: %v", requestUrl, http.MethodGet))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodGet, requestUrl, nil, "")
	if err != nil {
		logger.Error("error on list storage groups",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodGet),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return nil, status, err
	}

	if status == http.StatusOK {
		storageGroups := &StorageGroupArray{}
		err := json.Unmarshal(responseBody, storageGroups)
		if err != nil {
			logger.Error("error on unmarshalling adapters",
				zap.String("request url", fmt.Sprint(requestUrl)),
				zap.String("method", http.MethodGet),
				zap.Error(fmt.Errorf("%v", getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err))))
			return nil, status, getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err)
		}
		logger.Info(fmt.Sprintf("Response: request url: %v, method: %v, status: %v, storage groups: %v", requestUrl, http.MethodGet, status, storageGroups.STORAGEGROUPS))
		return storageGroups.STORAGEGROUPS, status, nil
	}
	errorResponseBody := GenerateErrorFromResponse(responseBody)
	logger.Error("error on list storage groups",
		zap.String("request url", fmt.Sprint(requestUrl)),
		zap.String("method", http.MethodGet),
		zap.String("status: ", fmt.Sprint(status)),
		zap.Error(fmt.Errorf("%v", errorResponseBody)))
	return nil, status, errorResponseBody
}

/**
 * GET /api/storage-groups/{storage-group-id}
 * @cpcURI the ID of the virtual switch
 * @return adapter array
 * Return: 200 and Storage Group object
 *     or: 400, 404, 409
 */
func (m *StorageGroupManager) GetStorageGroupProperties(storageGroupURI string) (*StorageGroupProperties, int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, storageGroupURI)

	logger.Info(fmt.Sprintf("Request URL: %v, Method: %v", requestUrl, http.MethodGet))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodGet, requestUrl, nil, "")
	if err != nil {
		logger.Error("error on get storage group properties",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodGet),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return nil, status, err
	}

	if status == http.StatusOK {
		storageGroup := &StorageGroupProperties{}
		err := json.Unmarshal(responseBody, storageGroup)
		if err != nil {
			logger.Error("error on unmarshalling adapters",
				zap.String("request url", fmt.Sprint(requestUrl)),
				zap.String("method", http.MethodGet),
				zap.Error(fmt.Errorf("%v", getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err))))
			return nil, status, getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err)
		}
		logger.Info(fmt.Sprintf("Response: request url: %v, method: %v, status: %v, storage group properties: %v", requestUrl, http.MethodGet, status, storageGroup))
		return storageGroup, status, nil
	}
	errorResponseBody := GenerateErrorFromResponse(responseBody)
	logger.Error("error on get storage group properties",
		zap.String("request url", fmt.Sprint(requestUrl)),
		zap.String("status: ", fmt.Sprint(status)),
		zap.Error(fmt.Errorf("%v", errorResponseBody)))
	return nil, status, errorResponseBody
}

/**
 * GET /api/storage-groups/{storage-group-id}/storage-volumes
 * @storage-group-id the Object id of the storage group
 * @return storage volume array
 * Return: 200 and Storage Group array
 *     or: 400, 404, 409
 */
func (m *StorageGroupManager) ListStorageVolumes(storageGroupURI string) ([]StorageVolume, int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, storageGroupURI)

	logger.Info(fmt.Sprintf("Request URL: %v, Method: %v", requestUrl, http.MethodGet))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodGet, requestUrl, nil, "")
	if err != nil {
		logger.Error("error on list storage volumes",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodGet),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return nil, status, err
	}

	if status == http.StatusOK {
		storageVolumes := &StorageVolumeArray{}
		err := json.Unmarshal(responseBody, storageVolumes)
		if err != nil {
			logger.Error("error on unmarshalling adapters",
				zap.String("request url", fmt.Sprint(requestUrl)),
				zap.String("method", http.MethodGet),
				zap.Error(fmt.Errorf("%v", getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err))))
			return nil, status, getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err)
		}
		logger.Info(fmt.Sprintf("Response: request url: %v, method: %v, status: %v, storage volumes: %v", requestUrl, http.MethodGet, status, storageVolumes.STORAGEVOLUMES))
		return storageVolumes.STORAGEVOLUMES, status, nil
	}
	errorResponseBody := GenerateErrorFromResponse(responseBody)
	logger.Error("error on list storage volumes",
		zap.String("request url", fmt.Sprint(requestUrl)),
		zap.String("method", http.MethodGet),
		zap.String("status: ", fmt.Sprint(status)),
		zap.Error(fmt.Errorf("%v", errorResponseBody)))
	return nil, status, errorResponseBody
}

/**
 * GET /api/storage-groups/{storage-group-id}/storage-volumes/{storage-volume-id}
 * @return volume object
 * Return: 200 and Storage Volume object
 *     or: 400, 404, 409
 */
func (m *StorageGroupManager) GetStorageVolumeProperties(storageVolumeURI string) (*StorageVolume, int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, storageVolumeURI)

	logger.Info(fmt.Sprintf("Request URL: %v, Method: %v", requestUrl, http.MethodGet))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodGet, requestUrl, nil, "")
	if err != nil {
		logger.Error("error on get storage volume properties",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodGet),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return nil, status, err
	}

	if status == http.StatusOK {
		storageVolume := &StorageVolume{}
		err := json.Unmarshal(responseBody, storageVolume)
		if err != nil {
			logger.Error("error on unmarshalling adapters",
				zap.String("request url", fmt.Sprint(requestUrl)),
				zap.String("method", http.MethodGet),
				zap.Error(fmt.Errorf("%v", getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err))))
			return nil, status, getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err)
		}
		logger.Info(fmt.Sprintf("Respone: request url: %v, method: %v, status: %v, storage volume properties: %v", http.MethodGet, requestUrl, status, storageVolume))
		return storageVolume, status, nil
	}
	errorResponseBody := GenerateErrorFromResponse(responseBody)
	logger.Error("error on get storage volume properties",
		zap.String("request url", fmt.Sprint(requestUrl)),
		zap.String("method", http.MethodGet),
		zap.String("status: ", fmt.Sprint(status)),
		zap.Error(fmt.Errorf("%v", errorResponseBody)))
	return nil, status, errorResponseBody
}

/**
 * POST /api/storage-groups/{storage-group-id}/operations/modify
 * Return: 200
 *     or: 400, 404, 409
 */
func (m *StorageGroupManager) UpdateStorageGroupProperties(storageGroupURI string, updateRequest *StorageGroupProperties) (int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, storageGroupURI)

	logger.Info(fmt.Sprintf("Request URL: %v,  Method: %v", requestUrl, http.MethodPost))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodPost, requestUrl, updateRequest, "")
	if err != nil {
		logger.Error("error on update storage group properties",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodPost),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return status, err
	}

	if status == http.StatusOK {
		storageGroup := &StorageGroup{}
		err := json.Unmarshal(responseBody, storageGroup)
		if err != nil {
			logger.Error("error on unmarshalling adapters",
				zap.String("request url", fmt.Sprint(requestUrl)),
				zap.String("method", http.MethodPost),
				zap.Error(fmt.Errorf("%v", getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err))))
			return status, getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err)
		}
		logger.Info(fmt.Sprintf("Response: update storage group properties completed, request url: %v, method: %v, status: %v", requestUrl, http.MethodPost, status))
		return status, nil
	}
	logger.Error("error on update storage group properties",
		zap.String("request url", fmt.Sprint(requestUrl)),
		zap.String("method", http.MethodPost),
		zap.String("status: ", fmt.Sprint(status)))
	return status, nil
}

/*
*
  - POST /api/storage-groups/{storage-group-id}/operations/accept-mismatched-
    storage-volumes
  - Return: 200
  - or: 400, 404, 409
*/
func (m *StorageGroupManager) FulfillStorageGroup(storageGroupURI string, request *StorageGroupProperties) (int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, storageGroupURI)

	logger.Info(fmt.Sprintf("Request URL: %v, Method: %v", requestUrl, http.MethodPost))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodPost, requestUrl, request, "")
	if err != nil {
		logger.Error("error on fulfill storage group",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodPost),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return status, err
	}

	if status == http.StatusOK {
		storageGroup := &StorageGroup{}
		err := json.Unmarshal(responseBody, storageGroup)
		if err != nil {
			logger.Error("error on unmarshalling adapters",
				zap.String("request url", fmt.Sprint(requestUrl)),
				zap.String("method", http.MethodPost),
				zap.Error(fmt.Errorf("%v", getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err))))
			return status, getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err)
		}
		logger.Info(fmt.Sprintf("Response: fulfill storage group completed, request url: %v, method: %v, status: %v", requestUrl, http.MethodPost, status))
		return status, nil
	}
	logger.Error("error on fulfill storage group",
		zap.String("request url", fmt.Sprint(requestUrl)),
		zap.String("method", http.MethodPost),
		zap.String("status: ", fmt.Sprint(status)))
	return status, nil
}

// CreateStorageGroup

/**
 * POST/api/storage-groups
 * @returns object-uri and the element-uri of each storage volume that was created in the response body
 * Return: 201
 *     or: 400, 404, 409
 */
func (m *StorageGroupManager) CreateStorageGroups(storageGroupURI string, storageGroup *CreateStorageGroupProperties) (*StorageGroupCreateResponse, int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, storageGroupURI)

	status, responseBody, err := m.client.ExecuteRequest(http.MethodPost, requestUrl, storageGroup, "")
	if err != nil {
		return nil, status, err
	}

	if status == http.StatusCreated {
		sgURI := StorageGroupCreateResponse{}
		err := json.Unmarshal(responseBody, &sgURI)
		if err != nil {
			return nil, status, getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err)
		}
		var svPaths []StorageGroupVolumePath
		for _, i := range sgURI.URI {
			sv, statuscode, err := m.GetStorageVolumeProperties(i)
			if err != nil {
				return nil, statuscode, err
			}
			svpath := StorageGroupVolumePath{
				URI:   sv.URI,
				Paths: sv.Paths,
			}
			svPaths = append(svPaths, svpath)
		}
		sgURI.SvPaths = svPaths
		return &sgURI, status, nil
	}

	return nil, status, GenerateErrorFromResponse(responseBody)
}

/**
 * GET /api/storage-groups/{storage-group-id}/operations/get-partitions
 * @cpcURI the ID of the virtual switch
 * @return adapter array
 * Return: 200 and Storage Group object
 *     or: 400, 404, 409
 */
func (m *StorageGroupManager) GetStorageGroupPartitions(storageGroupURI string, query map[string]string) (*StorageGroupPartitions, int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, storageGroupURI, "/operations/get-partitions")

	requestUrl = BuildUrlFromQuery(requestUrl, query)

	status, responseBody, err := m.client.ExecuteRequest(http.MethodGet, requestUrl, nil, "")

	if err != nil {
		return nil, status, err
	}

	if status == http.StatusOK {
		storageGroup := StorageGroupPartitions{}

		err := json.Unmarshal(responseBody, &storageGroup)

		if err != nil {
			return nil, status, getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err)
		}
		return &storageGroup, status, nil
	}

	return nil, status, GenerateErrorFromResponse(responseBody)
}

func (m *StorageGroupManager) DeleteStorageGroup(storageGroupURI string) (int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, storageGroupURI, "/operations/delete")
	status, responseBody, err := m.client.ExecuteRequest(http.MethodPost, requestUrl, nil, "")
	if err != nil {
		return status, err
	}
	if status == http.StatusNoContent {
		return status, nil
	}

	errorResponseBody := GenerateErrorFromResponse(responseBody)
	logger.Error("error deleting storage group ",
		zap.String("request url", fmt.Sprint(storageGroupURI)),
		zap.String("method", http.MethodPost),
		zap.String("status: ", fmt.Sprint(status)),
		zap.Error(fmt.Errorf("%v", errorResponseBody)))
	return status, errorResponseBody

}
