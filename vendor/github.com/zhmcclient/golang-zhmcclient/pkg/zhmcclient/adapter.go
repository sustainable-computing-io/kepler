/*
 * =============================================================================
 * IBM Confidential
 * © Copyright IBM Corp. 2020, 2021
 *
 * The source code for this program is not published or otherwise divested of
 * its trade secrets, irrespective of what has been deposited with the
 * U.S. Copyright Office.
 * =============================================================================
 */

package zhmcclient

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path"

	"go.uber.org/zap"
)

// AdapterAPI defines an interface for issuing Adapter requests to ZHMC
//
//go:generate counterfeiter -o fakes/adapter.go --fake-name AdapterAPI . AdapterAPI
type AdapterAPI interface {
	ListAdapters(cpcURI string, query map[string]string) ([]Adapter, int, *HmcError)
	GetAdapterProperties(adapterURI string) (*AdapterProperties, int, *HmcError)
	GetNetworkAdapterPortProperties(networkAdapterPortURI string) (*NetworkAdapterPort, int, *HmcError)
	GetStorageAdapterPortProperties(storageAdapterPortURI string) (*StorageAdapterPort, int, *HmcError)
	CreateHipersocket(cpcURI string, adaptor *HipersocketPayload) (string, int, *HmcError)
	DeleteHipersocket(adapterURI string) (int, *HmcError)
}

type AdapterManager struct {
	client ClientAPI
}

func NewAdapterManager(client ClientAPI) *AdapterManager {
	return &AdapterManager{
		client: client,
	}
}

/**
* GET /api/cpcs/{cpc-id}/adapters
* @cpcURI the ID of the CPC
* @query the fields can be queried include:
*        name,
*        adapter-id,
*        adapter-family,
*        type,
*        status
* @return adapter array
* Return: 200 and Adapters array
*     or: 400, 404, 409
 */
func (m *AdapterManager) ListAdapters(cpcURI string, query map[string]string) ([]Adapter, int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, cpcURI, "/adapters")
	requestUrl = BuildUrlFromQuery(requestUrl, query)
	logger.Info(fmt.Sprintf("Request URL: %v, Method: %v", requestUrl, http.MethodGet))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodGet, requestUrl, nil, "")
	if err != nil {
		logger.Error("error on listing adapters",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodGet),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return nil, status, err
	}
	logger.Info(fmt.Sprintf("Response: listing adapters, request url: %v, method: %v, status: %v", requestUrl, http.MethodGet, status))

	if status == http.StatusOK {
		adapters := &AdaptersArray{}
		err := json.Unmarshal(responseBody, adapters)
		if err != nil {
			logger.Error("error on unmarshalling adapters",
				zap.String("request url", fmt.Sprint(requestUrl)),
				zap.String("method", http.MethodGet),
				zap.Error(fmt.Errorf("%v", getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err))))
			return nil, status, getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err)
		}
		return adapters.ADAPTERS, status, nil
	}
	errorResponseBody := GenerateErrorFromResponse(responseBody)
	logger.Error("error listing adapters",
		zap.String("request url", fmt.Sprint(requestUrl)),
		zap.String("method", http.MethodGet),
		zap.String("status: ", fmt.Sprint(status)),
		zap.Error(fmt.Errorf("%v", errorResponseBody)))

	return nil, status, errorResponseBody
}

/**
* GET /api/adapters/{adapter-id}
* GET /api/adapters/{adapter-id}/network-ports/{network-port-id}
* @adapterURI the adapter ID, network-port-id for which properties need to be fetched
* @return adapter properties
* Return: 200 and Adapters properties
*     or: 400, 404, 409
 */
func (m *AdapterManager) GetAdapterProperties(adapterURI string) (*AdapterProperties, int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, adapterURI)

	logger.Info(fmt.Sprintf("Request URL: %v, Method: %v", requestUrl, http.MethodGet))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodGet, requestUrl, nil, "")
	if err != nil {
		logger.Error("error on getting adapter properties",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodGet),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return nil, status, err
	}

	if status == http.StatusOK {
		adapterProps := &AdapterProperties{}
		err := json.Unmarshal(responseBody, adapterProps)
		if err != nil {
			logger.Error("error on unmarshalling adapters",
				zap.String("request url", fmt.Sprint(requestUrl)),
				zap.String("method", http.MethodGet),
				zap.Error(fmt.Errorf("%v", getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err))))
			return nil, status, getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err)
		}
		logger.Info(fmt.Sprintf("Response: request url: %v, method: %v, status: %v, adapters: %v", requestUrl, http.MethodGet, status, adapterProps))
		return adapterProps, status, nil
	}
	errorResponseBody := GenerateErrorFromResponse(responseBody)
	logger.Error("error getting adapter properties",
		zap.String("request url", fmt.Sprint(requestUrl)),
		zap.String("method", http.MethodGet),
		zap.String("status: ", fmt.Sprint(status)),
		zap.Error(fmt.Errorf("%v", errorResponseBody)))
	return nil, status, errorResponseBody
}

/**
* GET /api/adapters/{adapter-id}/storage-ports/{storage-port-id}
* @adapterURI the adapter ID, storage-port-id for which properties need to be fetched
* @return storage port properties
* Return: 200 and StorageAdapterPort
*     or: 400, 404, 409
 */
func (m *AdapterManager) GetStorageAdapterPortProperties(storageAdapterPortURI string) (*StorageAdapterPort, int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, storageAdapterPortURI)

	logger.Info(fmt.Sprintf("Request URL: %v, Method: %v", requestUrl, http.MethodGet))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodGet, requestUrl, nil, "")
	if err != nil {
		logger.Error("error on getting storage port properties",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodGet),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return nil, status, err
	}

	if status == http.StatusOK {
		portProps := &StorageAdapterPort{}
		err := json.Unmarshal(responseBody, portProps)
		if err != nil {
			logger.Error("error on unmarshalling storage port",
				zap.String("request url", fmt.Sprint(requestUrl)),
				zap.String("method", http.MethodGet),
				zap.Error(fmt.Errorf("%v", getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err))))
			return nil, status, getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err)
		}
		logger.Info(fmt.Sprintf("Response: request url: %v, method: %v, status: %v, storage port properties: %v", requestUrl, http.MethodGet, status, portProps))
		return portProps, status, nil
	}
	errorResponseBody := GenerateErrorFromResponse(responseBody)
	logger.Error("error getting storage port properties",
		zap.String("request url", fmt.Sprint(requestUrl)),
		zap.String("method", http.MethodGet),
		zap.String("status: ", fmt.Sprint(status)),
		zap.Error(fmt.Errorf("%v", errorResponseBody)))
	return nil, status, errorResponseBody
}

/**
* GET /api/adapters/{adapter-id}/network-ports/{network-port-id}
* @adapterURI the adapter ID, network-port-id for which properties need to be fetched
* @return network port properties
* Return: 200 and NetworkAdapterPort
*     or: 400, 404, 409
 */
func (m *AdapterManager) GetNetworkAdapterPortProperties(networkAdapterPortURI string) (*NetworkAdapterPort, int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, networkAdapterPortURI)

	logger.Info(fmt.Sprintf("Request URL: %v, Method: %v", requestUrl, http.MethodGet))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodGet, requestUrl, nil, "")
	if err != nil {
		logger.Error("error on getting network port properties",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodGet),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return nil, status, err
	}

	if status == http.StatusOK {
		portProps := &NetworkAdapterPort{}
		err := json.Unmarshal(responseBody, portProps)
		if err != nil {
			logger.Error("error on unmarshalling network port",
				zap.String("request url", fmt.Sprint(requestUrl)),
				zap.String("method", http.MethodGet),
				zap.Error(fmt.Errorf("%v", getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err))))
			return nil, status, getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err)
		}
		logger.Info(fmt.Sprintf("Response: request url: %v, method: %v, status: %v, network port properties: %v", requestUrl, http.MethodGet, status, portProps))
		return portProps, status, nil
	}
	errorResponseBody := GenerateErrorFromResponse(responseBody)
	logger.Error("error getting network port properties",
		zap.String("request url", fmt.Sprint(requestUrl)),
		zap.String("method", http.MethodGet),
		zap.String("status: ", fmt.Sprint(status)),
		zap.Error(fmt.Errorf("%v", errorResponseBody)))
	return nil, status, errorResponseBody
}

/**
* POST /api/cpcs/{cpc-id}/adapters
* @cpcURI the ID of the CPC
* @adaptor the payload includes properties when create Hipersocket
* Return: 201 and body with "object-uri"
*     or: 400, 403, 404, 409, 503
 */
func (m *AdapterManager) CreateHipersocket(cpcURI string, adaptor *HipersocketPayload) (string, int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, cpcURI, "/adapters")

	logger.Info(fmt.Sprintf("Request URL: %v,  Method: %v, Parameter: %v", requestUrl, http.MethodPost, adaptor))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodPost, requestUrl, adaptor, "")
	if err != nil {
		logger.Error("error creating hipersocket",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodPost),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return "", status, err
	}

	if status == http.StatusCreated {
		uriObj := HipersocketCreateResponse{}
		err := json.Unmarshal(responseBody, &uriObj)
		if err != nil {
			logger.Error("error on unmarshalling adapters",
				zap.String("request url", fmt.Sprint(requestUrl)),
				zap.String("method", http.MethodPost),
				zap.Error(fmt.Errorf("%v", getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err))))
			return "", status, getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err)
		}
		logger.Info(fmt.Sprintf("Response: hiper socket created, request url: %v, method: %v, status: %v, hipersocket uri: %v", requestUrl, http.MethodPost, status, uriObj.URI))

		return uriObj.URI, status, nil
	}

	errorResponseBody := GenerateErrorFromResponse(responseBody)
	logger.Error("error creating hipersocket",
		zap.String("request url", fmt.Sprint(requestUrl)),
		zap.String("method", http.MethodPost),
		zap.String("status: ", fmt.Sprint(status)),
		zap.Error(fmt.Errorf("%v", errorResponseBody)))
	return "", status, errorResponseBody
}

/**
* DELETE /api/adapters/{adapter-id}
* @adapterURI the adapter ID to be deleted
* Return: 204
*     or: 400, 403, 404, 409, 503
 */
func (m *AdapterManager) DeleteHipersocket(adapterURI string) (int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, adapterURI)

	logger.Info(fmt.Sprintf("Request URL: %v, Method: %v", requestUrl, http.MethodDelete))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodDelete, requestUrl, nil, "")
	if err != nil {
		logger.Error("error deleting hipersocket",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodDelete),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return status, err
	}

	if status == http.StatusNoContent {
		logger.Info(fmt.Sprintf("Response: hipersocket deleted, request url: %v, method: %v, status: %v", requestUrl, http.MethodDelete, status))
		return status, nil
	}
	errorResponseBody := GenerateErrorFromResponse(responseBody)
	logger.Error("error deleting hipersocket",
		zap.String("request url", fmt.Sprint(requestUrl)),
		zap.String("method", http.MethodDelete),
		zap.String("status: ", fmt.Sprint(status)),
		zap.Error(fmt.Errorf("%v", errorResponseBody)))
	return status, errorResponseBody
}
