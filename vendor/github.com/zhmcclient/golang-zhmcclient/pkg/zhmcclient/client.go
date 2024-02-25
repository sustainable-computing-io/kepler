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
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path"
	"strings"

	"net/http"
	"net/http/httputil"

	"net/url"

	"go.uber.org/zap"
)

func NewZapLogger() Logger {
	zapLogger, _ := zap.NewProduction()
	return zapLogger
}

var logger = NewZapLogger()

/*
	ClientAPI defines an interface for issuing client requests to ZHMC
*/

//go:generate counterfeiter -o fakes/client.go --fake-name ClientAPI . ClientAPI
type ClientAPI interface {
	CloneEndpointURL() *url.URL
	TraceOn(outputStream io.Writer)
	TraceOff()
	SetSkipCertVerify(isSkipCert bool)
	Logon() *HmcError
	Logoff() *HmcError
	GetMetricsContext() *MetricsContextDef
	LogonConsole() (string, int, *HmcError)
	LogoffConsole(sessionID string) *HmcError
	IsLogon(verify bool) bool
	ExecuteRequest(requestType string, url *url.URL, requestData interface{}, sessionID string) (responseStatusCode int, responseBodyStream []byte, err *HmcError)
	UploadRequest(requestType string, url *url.URL, requestData []byte) (responseStatusCode int, responseBodyStream []byte, err *HmcError)
}

// HTTP Client interface required for unit tests
type HTTPClient interface {
	Do(request *http.Request) (*http.Response, error)
}

const (
	SESSION_HEADER_NAME = "X-API-Session"
	minHMCMetricsSampleInterval = 15 //seconds
	metricsContextCreationURI = "/api/services/metrics/context"
)

type Options struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Trace    bool   `json:"trace,omitempty"`
	CaCert   string `json:"ca-cert,omitempty"`
	SkipCert bool   `json:"skip-cert,omitempty"`
}

type LogonData struct {
	Userid   string `json:"userid"`
	Password string `json:"password"`
}

type ChangePasswordData struct {
	Userid      string `json:"userid"`
	Password    string `json:"password"`
	NewPassword string `json:"new-password"`
}

// TODO, Use cache and use JobTopic, ObjectTopic to update cache
type Session struct {
	MajorVersion int    `json:"api-major-version,omitempty"`
	MinorVersion int    `json:"api-minor-version,omitempty"`
	SessionID    string `json:"api-session,omitempty"`
	JobTopic     string `json:"job-notification-topic,omitempty"`
	ObjectTopic  string `json:"notification-topic,omitempty"`
	Expires      int    `json:"password-expires,omitempty"`
	Credential   string `json:"session-credential,omitempty"`
}

type Client struct {
	endpointURL    *url.URL
	httpClient     *http.Client
	logondata      *LogonData
	session        *Session
	metricsContext *MetricsContextDef
	isSkipCert     bool
	isTraceEnabled bool
	traceOutput    io.Writer
}

func newClientStruct(endpoint string, opts *Options) (*Client, *HmcError) {
	tslConfig, err := SetCertificate(opts, &tls.Config{})
	if err != nil {
		return nil, err
	}
	tslConfig.InsecureSkipVerify = opts.SkipCert
	transport := &http.Transport{
		Dial: (&net.Dialer{
			Timeout: DEFAULT_DIAL_TIMEOUT,
		}).Dial,
		TLSClientConfig:     tslConfig,
		TLSHandshakeTimeout: DEFAULT_HANDSHAKE_TIMEOUT,
	}

	httpclient := &http.Client{
		Timeout:   DEFAULT_CONNECT_TIMEOUT,
		Transport: transport,
	}

	endpointurl, err := GetEndpointURLFromString(endpoint)
	if err != nil {
		return nil, err
	}

	client := &Client{
		endpointURL: endpointurl,
		httpClient:  httpclient,
		logondata: &LogonData{
			Userid:   opts.Username,
			Password: opts.Password,
		},
	}
	return client, nil
}

func NewClient(endpoint string, opts *Options, l Logger) (ClientAPI, *HmcError) {

	if l != nil {
		logger = l
	}

	client, err := newClientStruct(endpoint, opts)
	if err != nil {
		return nil, err
	}

	err = client.Logon()
	if err != nil {
		return nil, err
	}

	if opts.Trace {
		client.TraceOn(nil)
	} else {
		client.TraceOff()
	}

	client.SetSkipCertVerify(opts.SkipCert)

	return client, nil
}

func GetEndpointURLFromString(endpoint string) (*url.URL, *HmcError) {
	logger.Info(fmt.Sprintf("endpoint: %v", endpoint))

	if !strings.HasPrefix(strings.ToLower(endpoint), "https") {
		return nil, getHmcErrorFromMsg(ERR_CODE_HMC_INVALID_URL, ERR_MSG_INSECURE_URL)
	}

	url, err := url.Parse(endpoint)
	if err != nil {
		return nil, getHmcErrorFromErr(ERR_CODE_HMC_INVALID_URL, err)
	}
	return url, nil
}

func IsExpectedHttpStatus(status int) bool {
	for _, httpStatus := range KNOWN_SUCCESS_STATUS {
		if httpStatus == status {
			return true
		}
	}
	return false
}

func NeedLogon(status, reason int) bool {
	if status == http.StatusUnauthorized {
		return true
	}

	if status == http.StatusForbidden {
		if reason == 4 || reason == 5 {
			return true
		}
	}
	return false
}

/**
* make a copy of the URL as it may be changed.
 */
func (c *Client) CloneEndpointURL() *url.URL {
	var url *url.URL
	if c.endpointURL == nil {
		return url
	}
	url, _ = url.Parse(c.endpointURL.String())
	return url
}

func (c *Client) TraceOn(outputStream io.Writer) {
	if outputStream == nil {
		outputStream = os.Stdout
	}
	c.traceOutput = outputStream
	c.isTraceEnabled = true
}

func (c *Client) TraceOff() {
	c.traceOutput = os.Stdout
	c.isTraceEnabled = false
}

// TODO, check cert when request/logon
func (c *Client) SetSkipCertVerify(isSkipCert bool) {
	c.isSkipCert = isSkipCert
}

func (c *Client) clearSession() {
	c.session = nil
}

func (c *Client) Logon() *HmcError {
	c.clearSession()
	url := c.CloneEndpointURL()
	if url == nil {
		return &HmcError{Reason: int(ERR_CODE_HMC_INVALID_URL), Message: ERR_MSG_EMPTY_JOB_URI}
	}
	url.Path = path.Join(url.Path, "/api/sessions")

	status, responseBody, hmcErr := c.executeMethod(http.MethodPost, url.String(), c.logondata, "")

	if hmcErr != nil {
		return hmcErr
	}

	if status == http.StatusOK || status == http.StatusCreated {
		session := &Session{}
		err := json.Unmarshal(responseBody, session)
		if err != nil {
			return getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err)
		}
		c.session = session
		metricsGroupList := []string{
			"logical-partition-usage",
		}
		if err = c.createMetricsContext(metricsGroupList); err != nil {
			return getHmcErrorFromErr(ERR_CODE_HMC_CREATE_METRICS_CTX_FAIL, err)
		}
		return nil
	}

	return GenerateErrorFromResponse(responseBody)
}

// login and change password, then end session
func ChangePassword(endpoint string, opts *Options, newPassword string) *HmcError {
	c, err := newClientStruct(endpoint, opts)
	if err != nil {
		return err
	}

	c.clearSession()
	url := c.CloneEndpointURL()
	if url == nil {
		return &HmcError{Reason: int(ERR_CODE_HMC_INVALID_URL), Message: ERR_MSG_EMPTY_JOB_URI}
	}
	url.Path = path.Join(url.Path, "/api/sessions")

	changePasswordData := ChangePasswordData{
		Userid:      c.logondata.Userid,
		Password:    c.logondata.Password,
		NewPassword: newPassword,
	}

	status, responseBody, hmcErr := c.executeMethod(http.MethodPost, url.String(), changePasswordData, "")

	defer c.Logoff()

	if hmcErr != nil {
		return hmcErr
	}

	if status == http.StatusOK || status == http.StatusCreated {
		session := &Session{}
		err := json.Unmarshal(responseBody, session)
		if err != nil {
			return getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err)
		}
		c.session = session
		return nil
	}

	return GenerateErrorFromResponse(responseBody)
}

func (c *Client) LogonConsole() (sessionID string, status int, err *HmcError) {
	url := c.CloneEndpointURL()
	url.Path = path.Join(url.Path, "/api/sessions")

	status, responseBody, err := c.executeMethod(http.MethodPost, url.String(), c.logondata, "")
	if err != nil {
		return "", status, err
	}
	if status == http.StatusOK || status == http.StatusCreated {
		session := &Session{}
		err := json.Unmarshal(responseBody, session)
		if err != nil {
			return "", status, getHmcErrorFromErr(ERR_CODE_HMC_UNMARSHAL_FAIL, err)
		}
		return session.SessionID, status, nil
	}

	return "", status, GenerateErrorFromResponse(responseBody)
}

func (c *Client) LogoffConsole(sessionID string) *HmcError {
	url := c.CloneEndpointURL()
	url.Path = path.Join(url.Path, "/api/sessions/this-session")

	status, responseBody, err := c.executeMethod(http.MethodDelete, url.String(), nil, sessionID)
	if err != nil {
		return err
	}
	if status == http.StatusNoContent {
		return nil
	}
	return GenerateErrorFromResponse(responseBody)
}

func (c *Client) Logoff() *HmcError {
	url := c.CloneEndpointURL()
	url.Path = path.Join(url.Path, "/api/sessions/this-session")

	status, responseBody, err := c.executeMethod(http.MethodDelete, url.String(), nil, "")
	if err != nil {
		return err
	}
	if status == http.StatusNoContent {
		c.clearSession()
		return nil
	}
	return GenerateErrorFromResponse(responseBody)
}

func (c *Client) IsLogon(verify bool) bool {
	if verify {
		url := c.CloneEndpointURL()
		if url == nil {
			return false
		}
		url.Path = path.Join(url.Path, "/api/console")

		status, _, err := c.executeMethod(http.MethodGet, url.String(), nil, "")
		if err != nil {
			return false
		} else if status == http.StatusOK || status == http.StatusBadRequest {
			return true
		}
		return false
	}

	if c.session != nil && c.session.SessionID != "" {
		return true
	}
	return false
}

func (c *Client) setUserAgent(req *http.Request) {
	req.Header.Set("User-Agent", libraryUserAgent)
}

func (c *Client) setRequestHeaders(req *http.Request, bodyType, sessionID string) {
	c.setUserAgent(req)
	req.Header.Add("Content-Type", bodyType)
	req.Header.Add("Accept", "*/*")
	if sessionID != "" {
		req.Header.Add(SESSION_HEADER_NAME, sessionID)
	} else if c.session != nil && c.session.SessionID != "" {
		req.Header.Add(SESSION_HEADER_NAME, c.session.SessionID)
	}
}

func SetCertificate(opts *Options, tlsConfig *tls.Config) (*tls.Config, *HmcError) {
	if !opts.SkipCert {
		//Read root CA bundle in PEM format
		cert, err := ioutil.ReadFile(opts.CaCert)
		if err != nil {
			return nil, getHmcErrorFromErr(ERR_CODE_HMC_READ_RESPONSE_FAIL, err)
		}
		if err != nil {
			return nil, getHmcErrorFromErr(ERR_CODE_HMC_BAD_REQUEST, err)
		}
		/*
			Read certs for PEM CA bundle and append rootCA cert pool of the TLS config for validation
		*/
		certPool := x509.NewCertPool()
		certPool.AppendCertsFromPEM(cert)
		tlsConfig.RootCAs = certPool
	}
	return tlsConfig, nil
}

func (c *Client) UploadRequest(requestType string, url *url.URL, requestData []byte) (responseStatusCode int, responseBodyStream []byte, err *HmcError) {
	retries := DEFAULT_CONNECT_RETRIES
	responseStatusCode, responseBodyStream, err = c.executeUpload(requestType, url.String(), requestData)
	if NeedLogon(responseStatusCode, GenerateErrorFromResponse(responseBodyStream).Reason) {
		c.Logon()
		c.executeUpload(requestType, url.String(), requestData)
	}
	if IsExpectedHttpStatus(responseStatusCode) {
		return
	}

	for retries > 0 {
		if IsExpectedHttpStatus(responseStatusCode) {
			break
		} else {
			logger.Info(fmt.Sprintf("Retry upload... %d", retries))
			responseStatusCode, responseBodyStream, err = c.executeUpload(requestType, url.String(), requestData)
			retries -= 1
		}
	}
	return responseStatusCode, responseBodyStream, err
}

func (c *Client) ExecuteRequest(requestType string, url *url.URL, requestData interface{}, sessionID string) (responseStatusCode int, responseBodyStream []byte, err *HmcError) {
	var (
		retries int
	)

	if requestType == http.MethodGet {
		retries = DEFAULT_READ_RETRIES

	} else {
		retries = DEFAULT_CONNECT_RETRIES
	}

	responseStatusCode, responseBodyStream, err = c.executeMethod(requestType, url.String(), requestData, sessionID)
	if NeedLogon(responseStatusCode, GenerateErrorFromResponse(responseBodyStream).Reason) {
		c.Logon()
		responseStatusCode, responseBodyStream, err = c.executeMethod(requestType, url.String(), requestData, sessionID)
	}
	if IsExpectedHttpStatus(responseStatusCode) { // Known error, don't retry
		return responseStatusCode, responseBodyStream, err
	}

	for retries > 0 {
		responseStatusCode, responseBodyStream, err = c.executeMethod(requestType, url.String(), requestData, sessionID)
		if IsExpectedHttpStatus(responseStatusCode) || err == nil { // 2. Known error, don't retry
			break
		} else { // 3. Retry
			retries -= 1
		}
	}
	return responseStatusCode, responseBodyStream, err
}

func (c *Client) executeMethod(requestType string, urlStr string, requestData interface{}, sessionID string) (responseStatusCode int, responseBodyStream []byte, hmcErr *HmcError) {
	var requestBody []byte
	var err error

	if requestData != nil {
		requestBody, err = json.Marshal(requestData)
		if err != nil {
			return -1, nil, getHmcErrorFromErr(ERR_CODE_HMC_MARSHAL_FAIL, err)
		}
	}

	request, err := http.NewRequest(requestType, urlStr, bytes.NewBuffer(requestBody))
	if err != nil {
		return -1, nil, getHmcErrorFromErr(ERR_CODE_HMC_BAD_REQUEST, err)
	}

	c.setRequestHeaders(request, APPLICATION_BODY_JSON, sessionID)

	response, err := c.httpClient.Do(request)

	if err != nil {
		return -1, nil, getHmcErrorFromErr(ERR_CODE_HMC_EXECUTE_FAIL, err)
	}
	if response == nil {
		return -1, nil, getHmcErrorFromMsg(ERR_CODE_HMC_EMPTY_RESPONSE, ERR_MSG_EMPTY_RESPONSE)
	}

	defer response.Body.Close()

	responseBodyStream, err = ioutil.ReadAll(response.Body)
	if err != nil {
		return -1, nil, getHmcErrorFromErr(ERR_CODE_HMC_READ_RESPONSE_FAIL, err)
	}

	if c.isTraceEnabled {
		err = c.traceHTTP(request, response)
		if err != nil {
			return response.StatusCode, nil, getHmcErrorFromErr(ERR_CODE_HMC_TRACE_REQUEST_FAIL, err)
		}
	}

	return response.StatusCode, responseBodyStream, nil
}

func (c *Client) executeUpload(requestType string, urlStr string, requestBody []byte) (responseStatusCode int, responseBodyStream []byte, hmcErr *HmcError) {

	request, err := http.NewRequest(requestType, urlStr, bytes.NewReader(requestBody))
	if err != nil {
		return -1, nil, getHmcErrorFromErr(ERR_CODE_HMC_BAD_REQUEST, err)
	}

	c.setRequestHeaders(request, APPLICATION_BODY_OCTET_STREAM, "")

	response, err := c.httpClient.Do(request)

	if response == nil {
		return -1, nil, getHmcErrorFromMsg(ERR_CODE_HMC_EMPTY_RESPONSE, ERR_MSG_EMPTY_RESPONSE)
	}

	if err != nil {
		return -1, nil, getHmcErrorFromErr(ERR_CODE_HMC_EXECUTE_FAIL, err)
	}

	defer response.Body.Close()

	responseBodyStream, err = ioutil.ReadAll(response.Body)
	if err != nil {
		return -1, nil, getHmcErrorFromErr(ERR_CODE_HMC_READ_RESPONSE_FAIL, err)
	}

	if c.isTraceEnabled {
		err = c.traceHTTP(request, response)
		if err != nil {
			return response.StatusCode, nil, getHmcErrorFromErr(ERR_CODE_HMC_TRACE_REQUEST_FAIL, err)
		}
	}

	return response.StatusCode, responseBodyStream, nil
}

func (c *Client) traceHTTP(req *http.Request, resp *http.Response) error {
	_, err := fmt.Fprintln(c.traceOutput, "---------START-HTTP---------")
	if err != nil {
		return err
	}

	reqTrace, err := httputil.DumpRequestOut(req, false)
	if err != nil {
		return err
	}

	_, err = fmt.Fprint(c.traceOutput, string(reqTrace))
	if err != nil {
		return err
	}

	var respTrace []byte
	if resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusPartialContent &&
		resp.StatusCode != http.StatusNoContent {
		respTrace, err = httputil.DumpResponse(resp, true)
		if err != nil {
			return err
		}
	} else {
		respTrace, err = httputil.DumpResponse(resp, false)
		if err != nil {
			return err
		}
	}

	_, err = fmt.Fprint(c.traceOutput, strings.TrimSuffix(string(respTrace), "\r\n"))
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(c.traceOutput, "---------END-HTTP---------")
	if err != nil {
		return err
	}

	return nil
}

// createMetricsContext creates a "Metrics Context" resource in the HMC.
// Metrics context is defintion of a set of metrics that will be collected.
func (c *Client) createMetricsContext(metricsGroups []string) error {

	reqBody := make(map[string]interface{})
	reqBody["anticipated-frequency-seconds"] = minHMCMetricsSampleInterval
	reqBody["metric-groups"] = metricsGroups

	requestUrl := c.CloneEndpointURL()
	requestUrl.Path = path.Join(requestUrl.Path, metricsContextCreationURI)
	_, mcResp, err := c.ExecuteRequest(http.MethodPost, requestUrl, reqBody, "")

	if err != nil {
		return err
	}

	c.metricsContext, _ = newMetricsContext(mcResp)
	logger.Info("Create 'MetricsContext'")
	
	return nil
}

func (c *Client) GetMetricsContext() *MetricsContextDef {
	return c.metricsContext
}
