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

package source

import (
	"fmt"
	"os"

	"k8s.io/klog/v2"
)

const (
	// Turn off telemetry
	off = iota
	// Turn on telemetry
	on
)

var (
	// List of QAT qatDevInfo for the device
	devices map[string]interface{}
)

type qatDevInfo struct {
	addr     string
	datafile *os.File
}

type QATTelemetry struct {
	collectionSupported bool
}

func (QATTelemetry) GetName() string {
	return "qat"
}

// Init initizalize and start the QAT metric collector
func (q *QATTelemetry) Init() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("could not init telemetry:%s", err)
		}
	}()

	// get qat devices
	devices, err = getDevices()
	if err != nil {
		q.collectionSupported = false
		return err
	}

	// turn on telemetry
	if err = controlTelemetry(devices, on); err != nil {
		klog.V(2).Infof("failed to start telemetry: %v\n", err)
		return err
	}

	// open the telemetry data file
	devices, err = openDataFile(devices)
	if err != nil {
		klog.V(2).Infof("failed to open telemetry data file: %v\n", err)
		return err
	}

	klog.Infof("found %d QAT devices\n", len(devices))
	q.collectionSupported = true
	return nil
}

// GetQATUtilization returns a map of ProcessUtilizationSample where the key is the qat device id
func (q *QATTelemetry) GetQATUtilization(devices map[string]interface{}) (map[string]DeviceUtilizationSample, error) {
	qatMetrics := map[string]DeviceUtilizationSample{}
	for qatDev, info := range devices {
		file := info.(qatDevInfo).datafile
		deviceUtil, err := getUtilization(file)
		if err != nil {
			klog.V(2).Infof("failed to get qat utilization on device %s: %v\n", qatDev, err)
			continue
		}
		qatMetrics[qatDev] = deviceUtil
	}
	return qatMetrics, nil
}

// Shutdown stops the qat metric collector
func (q *QATTelemetry) Shutdown() bool {
	var err error
	// close telemetry data file
	if err = closeDataFile(); err != nil {
		return false
	}
	// turn off telemetry
	if err = controlTelemetry(devices, off); err != nil {
		return false
	}
	return true
}

// GetQats returns a map with qat devices
func (q *QATTelemetry) GetQATs() map[string]interface{} {
	return devices
}

func (q *QATTelemetry) IsQATCollectionSupported() bool {
	return q.collectionSupported
}

func (q *QATTelemetry) SetQATCollectionSupported(supported bool) {
	q.collectionSupported = supported
}
