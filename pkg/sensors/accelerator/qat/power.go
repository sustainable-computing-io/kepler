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

package qat

import (
	"fmt"
	"sync"

	"github.com/sustainable-computing-io/kepler/pkg/config"
	qat_source "github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator/qat/source"
	"k8s.io/klog/v2"
)

var (
	qatImpl qatInterface
	errLib  = fmt.Errorf("could not start accelerator-qat collector")
	qatOnce sync.Once
)

type qatInterface interface {
	// Init initizalize and start the QAT metric collector
	Init() error
	// Shutdown stops the QAT metric collector
	Shutdown() bool
	// GetGpus returns a map with QAT device
	GetQATs() map[string]interface{}
	// GetQATUtilization returns a map of ProcessUtilizationSample where the key is the qat device id
	GetQATUtilization(device map[string]interface{}) (map[string]qat_source.DeviceUtilizationSample, error)
	// IsQATCollectionSupported returns if it is possible to use this collector
	IsQATCollectionSupported() bool
	// SetQATCollectionSupported manually set if it is possible to use this collector. This is for testing purpose only.
	SetQATCollectionSupported(bool)
}

// Init() only returns the erro regarding if the gpu collector was suceffully initialized or not
// The qat.go file has an init function that starts and configures the qat collector
// However this file is only included in the build if kepler is run with Intel QAT driver support.
func Init() error {
	qatOnce.Do(func() {
		if config.IsExposeQATMetricsEnabled() {
			qatImpl = &qat_source.QATTelemetry{}
			errLib = qatImpl.Init()
			if errLib == nil {
				klog.Infoln("Using qat-telemetry to obtain qat metrics")
				// If the library was successfully initialized, we don't need to return an error in the Init() function
				return
			}
			klog.Infof("Failed to init qat-telemtry err: %v\n", errLib)
		}
	})
	return errLib
}

func Shutdown() bool {
	if qatImpl != nil && config.IsExposeQATMetricsEnabled() {
		return qatImpl.Shutdown()
	}
	return true
}

func GetQATs() map[string]interface{} {
	if qatImpl != nil && config.IsExposeQATMetricsEnabled() {
		return qatImpl.GetQATs()
	}
	return map[string]interface{}{}
}

func GetQATUtilization(devices map[string]interface{}) (map[string]qat_source.DeviceUtilizationSample, error) {
	if qatImpl != nil && config.IsExposeQATMetricsEnabled() {
		deviceUtilization, err := qatImpl.GetQATUtilization(devices)
		if err == nil {
			return deviceUtilization, nil
		} else {
			klog.Infof("failed to collector QAT metrics, trying to initizalize again: %v \n", err)
			if errLib != nil {
				klog.Infof("failed to init qat-telemetry:%v\n", errLib)
			}
		}
	}
	return map[string]qat_source.DeviceUtilizationSample{}, errLib
}

func IsQATCollectionSupported() bool {
	if qatImpl != nil && config.IsExposeQATMetricsEnabled() {
		return qatImpl.IsQATCollectionSupported()
	}
	return false
}

func SetQATCollectionSupported(supported bool) {
	if qatImpl != nil && config.IsExposeQATMetricsEnabled() {
		qatImpl.SetQATCollectionSupported(supported)
	}
}
