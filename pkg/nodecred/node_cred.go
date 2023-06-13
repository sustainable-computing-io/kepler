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

// per node credential interface
// Kubernetes doesn't have per node API object, so we need to use node credential to access node specific information
// e.g. node specific metrics, node specific power consumption, etc.
// This interface is used to access node credential.
// Instances of node credentials can be ConfigMap, Secret, key value store, etc.
package nodecred

import (
	"fmt"

	"k8s.io/klog/v2"
)

type NodeCredInterface interface {
	// GetNodeCredByNodeName returns map of per node credential for targets such as redfish
	GetNodeCredByNodeName(nodeName string, target string) (map[string]string, error)
	// IsSupported returns if this node credential is supported
	IsSupported(info map[string]string) bool
}

var (
	// csvNodeCredImpl is the implementation of NodeCred using on disk csv file
	csvNodeCredImpl csvNodeCred
	// nodeCredImpl is the pointer to the runtime detected implementation of NodeCred
	nodeCredImpl NodeCredInterface = nil
)

func InitNodeCredImpl(param map[string]string) error {
	if csvNodeCredImpl.IsSupported(param) {
		klog.V(1).Infoln("use csv file to obtain node credential")
		nodeCredImpl = csvNodeCredImpl
	}
	if nodeCredImpl != nil {
		return nil
	}
	return fmt.Errorf("no supported node credential implementation")
}

func GetNodeCredByNodeName(nodeName, target string) (map[string]string, error) {
	return nodeCredImpl.GetNodeCredByNodeName(nodeName, target)
}
