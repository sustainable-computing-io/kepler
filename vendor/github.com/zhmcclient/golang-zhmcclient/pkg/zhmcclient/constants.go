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
	"net/http"
	"runtime"
	"time"
)

// Global constants.
const (
	libraryName            = "zhmcclient"
	libraryVersion         = "v0.1"
	libraryUserAgentPrefix = "ZHMC (" + runtime.GOOS + "; " + runtime.GOARCH + ") "
	libraryUserAgent       = libraryUserAgentPrefix + libraryName + "/" + libraryVersion

	HMC_DEFAULT_PORT = "6794"

	DEFAULT_READ_RETRIES    = 0
	DEFAULT_CONNECT_RETRIES = 3

	DEFAULT_DIAL_TIMEOUT      = 10 * time.Second
	DEFAULT_HANDSHAKE_TIMEOUT = 10 * time.Second
	DEFAULT_CONNECT_TIMEOUT   = 90 * time.Second
	DEFAULT_READ_TIMEOUT      = 3600 * time.Second
	DEFAULT_MAX_REDIRECTS     = 30 * time.Second
	DEFAULT_OPERATION_TIMEOUT = 3600 * time.Second
	DEFAULT_STATUS_TIMEOUT    = 900 * time.Second

	APPLICATION_BODY_JSON         = "application/json"
	APPLICATION_BODY_OCTET_STREAM = "application/octet-stream"
)

// List of success status.
var KNOWN_SUCCESS_STATUS = []int{
	http.StatusOK,             // 200
	http.StatusCreated,        // 201
	http.StatusAccepted,       // 202
	http.StatusNoContent,      // 204
	http.StatusPartialContent, // 206
	http.StatusBadRequest,     // 400
	//http.StatusForbidden,    // 403, 403 susally caused by expired session header, we need handle it separately
	http.StatusNotFound,            // 404
	http.StatusConflict,            // 409
	http.StatusInternalServerError, // 500
	http.StatusServiceUnavailable,  // 503
}
