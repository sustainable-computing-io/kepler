// Copyright 2016-2021 IBM Corp. All Rights Reserved.
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

// JobAPI defines an interface for issuing Job requests to ZHMC
//
//go:generate counterfeiter -o fakes/job.go --fake-name JobAPI . JobAPI
type JobAPI interface {
	QueryJob(jobURI string) (*Job, int, *HmcError)
	DeleteJob(jobURI string) (int, *HmcError)
	CancelJob(jobURI string) (int, *HmcError)
}

type JobManager struct {
	client ClientAPI
}

func NewJobManager(client ClientAPI) *JobManager {
	return &JobManager{
		client: client,
	}
}

/**
* GET /api/jobs/{job-id}
* Return: 200 and job status
*     or: 400, 404
 */
func (m *JobManager) QueryJob(jobURI string) (*Job, int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, jobURI)

	logger.Info(fmt.Sprintf("Request URL: %v, Method: %v", requestUrl, http.MethodGet))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodGet, requestUrl, nil, "")
	if err != nil {
		logger.Error("error on get on job uri",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodGet),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return nil, status, err
	}

	if status == http.StatusOK {
		myjob := Job{}
		err := json.Unmarshal(responseBody, &myjob)
		if err != nil {
			return nil, status, getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err)
		}
		logger.Info(fmt.Sprintf("Response: request url: %v, method: %v, status: %v, job: %v", requestUrl, http.MethodGet, status, &myjob))
		return &myjob, status, nil
	}
	errorResponseBody := GenerateErrorFromResponse(responseBody)
	logger.Error("error on get on job uri",
		zap.String("request url", fmt.Sprint(requestUrl)),
		zap.String("method", http.MethodGet),
		zap.String("status: ", fmt.Sprint(status)),
		zap.Error(fmt.Errorf("%v", errorResponseBody)))
	return nil, status, errorResponseBody
}

/**
* DELETE /api/jobs/{job-id}
* Return: 204
*     or: 400, 404, 409
 */
func (m *JobManager) DeleteJob(jobURI string) (int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, jobURI)

	logger.Info(fmt.Sprintf("Request URL: %v, Method: %v", requestUrl, http.MethodDelete))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodDelete, requestUrl, nil, "")
	if err != nil {
		logger.Error("error on delete job",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodDelete),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return status, err
	}

	if status == http.StatusNoContent {
		logger.Info(fmt.Sprintf("Response: job deleted, request url: %v, method: %v, status: %v", requestUrl, http.MethodDelete, status))
		return status, nil
	}
	errorResponseBody := GenerateErrorFromResponse(responseBody)
	logger.Error("error on delete job",
		zap.String("request url", fmt.Sprint(requestUrl)),
		zap.String("method", http.MethodDelete),
		zap.String("status: ", fmt.Sprint(status)),
		zap.Error(fmt.Errorf("%v", errorResponseBody)))
	return status, errorResponseBody
}

/**
* POST /api/jobs/{job-id}/operations/cancel
* Return: 204
*     or: 400, 404, 409
 */
func (m *JobManager) CancelJob(jobURI string) (int, *HmcError) {
	requestUrl := m.client.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, jobURI, "operations/cancel")

	logger.Info(fmt.Sprintf("Request URL: %v, Method: %v", requestUrl, http.MethodPost))
	status, responseBody, err := m.client.ExecuteRequest(http.MethodPost, requestUrl, nil, "")
	if err != nil {
		logger.Error("error on cancel job",
			zap.String("request url", fmt.Sprint(requestUrl)),
			zap.String("method", http.MethodPost),
			zap.String("status", fmt.Sprint(status)),
			zap.Error(fmt.Errorf("%v", err)))
		return status, err
	}

	if status == http.StatusNoContent {
		logger.Info(fmt.Sprintf("Response: job cancelled, request url: %v, method: %v, status: %v", requestUrl, http.MethodPost, status))
		return status, nil
	}
	errorResponseBody := GenerateErrorFromResponse(responseBody)
	logger.Error("error on cancel job",
		zap.String("request url", fmt.Sprint(requestUrl)),
		zap.String("method", http.MethodPost),
		zap.String("status: ", fmt.Sprint(status)),
		zap.Error(fmt.Errorf("%v", errorResponseBody)))
	return status, errorResponseBody
}
