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

// ZhmcAPI defines an interface for issuing requests to ZHMC
//
//go:generate counterfeiter -o fakes/zhmc.go --fake-name ZhmcAPI . ZhmcAPI
type ZhmcAPI interface {
	CpcAPI
	LparAPI
	NicAPI
	AdapterAPI
	StorageGroupAPI
	VirtualSwitchAPI
	JobAPI
	MetricsAPI
}

type ZhmcManager struct {
	client               ClientAPI
	cpcManager           CpcAPI
	lparManager          LparAPI
	adapterManager       AdapterAPI
	storageGroupManager  StorageGroupAPI
	virtualSwitchManager VirtualSwitchAPI
	nicManager           NicAPI
	jobManager           JobAPI
	metricsManager       MetricsAPI
}

func NewManagerFromOptions(endpoint string, creds *Options, logger Logger) ZhmcAPI {
	client, _ := NewClient(endpoint, creds, logger)
	if client != nil {
		return NewManagerFromClient(client)
	}
	return nil
}

func NewManagerFromClient(client ClientAPI) ZhmcAPI {
	return &ZhmcManager{
		client:               client,
		cpcManager:           NewCpcManager(client),
		lparManager:          NewLparManager(client),
		adapterManager:       NewAdapterManager(client),
		storageGroupManager:  NewStorageGroupManager(client),
		virtualSwitchManager: NewVirtualSwitchManager(client),
		nicManager:           NewNicManager(client),
		jobManager:           NewJobManager(client),
		metricsManager:       NewMetricsManager(client),
	}
}

// CPC
func (m *ZhmcManager) ListCPCs(query map[string]string) ([]CPC, int, *HmcError) {
	return m.cpcManager.ListCPCs(query)
}
func (m *ZhmcManager) GetCPCProperties(cpcURI string) (*CPCProperties, int, *HmcError) {
	return m.cpcManager.GetCPCProperties(cpcURI)
}

// LPAR
func (m *ZhmcManager) ListLPARs(cpcURI string, query map[string]string) ([]LPAR, int, *HmcError) {
	return m.lparManager.ListLPARs(cpcURI, query)
}
func (m *ZhmcManager) GetLparProperties(lparURI string) (*LparObjectProperties, int, *HmcError) {
	return m.lparManager.GetLparProperties(lparURI)
}
func (m *ZhmcManager) UpdateLparProperties(lparURI string, props *LparProperties) (int, *HmcError) {
	return m.lparManager.UpdateLparProperties(lparURI, props)
}
func (m *ZhmcManager) CreateLPAR(cpcURI string, props *LparProperties) (string, int, *HmcError) {
	return m.lparManager.CreateLPAR(cpcURI, props)
}
func (m *ZhmcManager) StartLPAR(lparURI string) (string, int, *HmcError) {
	return m.lparManager.StartLPAR(lparURI)
}
func (m *ZhmcManager) StopLPAR(lparURI string) (string, int, *HmcError) {
	return m.lparManager.StopLPAR(lparURI)
}
func (m *ZhmcManager) DeleteLPAR(lparURI string) (int, *HmcError) {
	return m.lparManager.DeleteLPAR(lparURI)
}
func (m *ZhmcManager) MountIsoImage(lparURI string, isoFile string, insFile string) (int, *HmcError) {
	return m.lparManager.MountIsoImage(lparURI, isoFile, insFile)
}
func (m *ZhmcManager) UnmountIsoImage(lparURI string) (int, *HmcError) {
	return m.lparManager.UnmountIsoImage(lparURI)
}
func (m *ZhmcManager) ListNics(lparURI string) ([]string, int, *HmcError) {
	return m.lparManager.ListNics(lparURI)
}

func (m *ZhmcManager) AttachStorageGroupToPartition(lparURI string, request *StorageGroupPayload) (int, *HmcError) {
	return m.lparManager.AttachStorageGroupToPartition(lparURI, request)
}

func (m *ZhmcManager) DetachStorageGroupToPartition(lparURI string, request *StorageGroupPayload) (int, *HmcError) {
	return m.lparManager.DetachStorageGroupToPartition(lparURI, request)
}

func (m *ZhmcManager) FetchAsciiConsoleURI(lparURI string, request *AsciiConsoleURIPayload) (*AsciiConsoleURIResponse, int, *HmcError) {
	return m.lparManager.FetchAsciiConsoleURI(lparURI, request)
}

func (m *ZhmcManager) GetEnergyDetailsforLPAR(lparURI string, props *EnergyRequestPayload) (uint64, int, *HmcError) {
	return m.lparManager.GetEnergyDetailsforLPAR(lparURI, props)
}

func (m *ZhmcManager) AttachCryptoToPartition(lparURI string, request *CryptoConfig) (int, *HmcError) {
	return m.lparManager.AttachCryptoToPartition(lparURI, request)
}

func (m *ZhmcManager) GetLiveEnergyDetailsforLPAR(lparURI string) (uint64, int, *HmcError) {
	return m.metricsManager.GetLiveEnergyDetailsforLPAR(lparURI)
}

// Adapter
func (m *ZhmcManager) ListAdapters(cpcURI string, query map[string]string) ([]Adapter, int, *HmcError) {
	return m.adapterManager.ListAdapters(cpcURI, query)
}
func (m *ZhmcManager) GetAdapterProperties(adapterURI string) (*AdapterProperties, int, *HmcError) {
	return m.adapterManager.GetAdapterProperties(adapterURI)
}
func (m *ZhmcManager) GetNetworkAdapterPortProperties(adapterURI string) (*NetworkAdapterPort, int, *HmcError) {
	return m.adapterManager.GetNetworkAdapterPortProperties(adapterURI)
}
func (m *ZhmcManager) GetStorageAdapterPortProperties(adapterURI string) (*StorageAdapterPort, int, *HmcError) {
	return m.adapterManager.GetStorageAdapterPortProperties(adapterURI)
}
func (m *ZhmcManager) CreateHipersocket(cpcURI string, adaptor *HipersocketPayload) (string, int, *HmcError) {
	return m.adapterManager.CreateHipersocket(cpcURI, adaptor)
}
func (m *ZhmcManager) DeleteHipersocket(adapterURI string) (int, *HmcError) {
	return m.adapterManager.DeleteHipersocket(adapterURI)
}

// Storage groups

func (m *ZhmcManager) ListStorageGroups(storageGroupURI string, cpc string) ([]StorageGroup, int, *HmcError) {
	return m.storageGroupManager.ListStorageGroups(storageGroupURI, cpc)
}

func (m *ZhmcManager) GetStorageGroupProperties(storageGroupURI string) (*StorageGroupProperties, int, *HmcError) {
	return m.storageGroupManager.GetStorageGroupProperties(storageGroupURI)
}

func (m *ZhmcManager) ListStorageVolumes(storageGroupURI string) ([]StorageVolume, int, *HmcError) {
	return m.storageGroupManager.ListStorageVolumes(storageGroupURI)
}

func (m *ZhmcManager) GetStorageVolumeProperties(storageGroupURI string) (*StorageVolume, int, *HmcError) {
	return m.storageGroupManager.GetStorageVolumeProperties(storageGroupURI)
}

func (m *ZhmcManager) UpdateStorageGroupProperties(storageGroupURI string, uploadRequest *StorageGroupProperties) (int, *HmcError) {
	return m.storageGroupManager.UpdateStorageGroupProperties(storageGroupURI, uploadRequest)
}

func (m *ZhmcManager) FulfillStorageGroup(storageGroupURI string, updateRequest *StorageGroupProperties) (int, *HmcError) {
	return m.storageGroupManager.FulfillStorageGroup(storageGroupURI, updateRequest)
}

func (m *ZhmcManager) CreateStorageGroups(storageGroupURI string, storageGroup *CreateStorageGroupProperties) (*StorageGroupCreateResponse, int, *HmcError) {
	return m.storageGroupManager.CreateStorageGroups(storageGroupURI, storageGroup)
}

func (m *ZhmcManager) GetStorageGroupPartitions(storageGroupURI string, query map[string]string) (*StorageGroupPartitions, int, *HmcError) {
	return m.storageGroupManager.GetStorageGroupPartitions(storageGroupURI, query)
}

func (m *ZhmcManager) DeleteStorageGroup(storageGroupURI string) (int, *HmcError) {
	return m.storageGroupManager.DeleteStorageGroup(storageGroupURI)
}

// Virtual Switches

func (m *ZhmcManager) ListVirtualSwitches(cpcURI string, query map[string]string) ([]VirtualSwitch, int, *HmcError) {
	return m.virtualSwitchManager.ListVirtualSwitches(cpcURI, query)
}

func (m *ZhmcManager) GetVirtualSwitchProperties(vSwitchURI string) (*VirtualSwitchProperties, int, *HmcError) {
	return m.virtualSwitchManager.GetVirtualSwitchProperties(vSwitchURI)
}

// NIC
func (m *ZhmcManager) CreateNic(lparURI string, nic *NIC) (string, int, *HmcError) {
	return m.nicManager.CreateNic(lparURI, nic)
}
func (m *ZhmcManager) DeleteNic(nicURI string) (int, *HmcError) {
	return m.nicManager.DeleteNic(nicURI)
}
func (m *ZhmcManager) GetNicProperties(nicURI string) (*NIC, int, *HmcError) {
	return m.nicManager.GetNicProperties(nicURI)
}
func (m *ZhmcManager) UpdateNicProperties(nicURI string, props *NIC) (int, *HmcError) {
	return m.nicManager.UpdateNicProperties(nicURI, props)
}

// JOB
func (m *ZhmcManager) QueryJob(jobURI string) (*Job, int, *HmcError) {
	return m.jobManager.QueryJob(jobURI)
}
func (m *ZhmcManager) DeleteJob(jobURI string) (int, *HmcError) {
	return m.jobManager.DeleteJob(jobURI)
}
func (m *ZhmcManager) CancelJob(jobURI string) (int, *HmcError) {
	return m.jobManager.CancelJob(jobURI)
}
