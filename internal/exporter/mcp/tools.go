// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sustainable-computing-io/kepler/internal/monitor"
)

// ListTopConsumersParams defines parameters for list_top_consumers tool
type ListTopConsumersParams struct {
	ResourceType string `json:"resource_type" jsonschema:"Resource type: node, process, container, vm, pod"`
	Limit        int    `json:"limit,omitempty" jsonschema:"Maximum number of results (default: 5)"`
	SortBy       string `json:"sort_by,omitempty" jsonschema:"Sort by power or energy (default: power)"`
}

// GetResourcePowerParams defines parameters for get_resource_power tool
type GetResourcePowerParams struct {
	ResourceType string `json:"resource_type" jsonschema:"Resource type: process, container, vm, pod"`
	ResourceID   string `json:"resource_id" jsonschema:"Resource identifier (PID for process, ID for others)"`
}

// SearchResourcesParams defines parameters for search_resources tool
type SearchResourcesParams struct {
	ResourceType string  `json:"resource_type" jsonschema:"Resource type: process, container, vm, pod"`
	PowerMin     float64 `json:"power_min,omitempty" jsonschema:"Minimum power consumption in watts"`
	PowerMax     float64 `json:"power_max,omitempty" jsonschema:"Maximum power consumption in watts"`
	NamePattern  string  `json:"name_pattern,omitempty" jsonschema:"Name pattern to match (substring search)"`
	Limit        int     `json:"limit,omitempty" jsonschema:"Maximum number of results (default: 10)"`
}

// PowerResourceInfo represents power consumption data for MCP responses
type PowerResourceInfo struct {
	Type        string             `json:"type"`
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Power       map[string]float64 `json:"power"`       // Zone -> Watts
	EnergyTotal map[string]float64 `json:"energyTotal"` // Zone -> Joules
	Metadata    map[string]string  `json:"metadata,omitempty"`
}

// handleListTopConsumers handles the list_top_consumers tool call
func (s *Server) handleListTopConsumers(ctx context.Context, cc *mcp.ServerSession, params *mcp.CallToolParamsFor[ListTopConsumersParams]) (*mcp.CallToolResultFor[any], error) {
	s.logger.Debug("Handling list_top_consumers request", "resource_type", params.Arguments.ResourceType)

	snapshot, err := s.monitor.Snapshot()
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}

	// Set defaults
	limit := params.Arguments.Limit
	if limit <= 0 {
		limit = 5
	}
	sortBy := params.Arguments.SortBy
	if sortBy == "" {
		sortBy = "power"
	}

	var resources []PowerResourceInfo
	switch params.Arguments.ResourceType {
	case "node":
		resources = s.convertNode(snapshot.Node)
	case "process":
		resources = s.convertProcesses(snapshot.Processes, limit, sortBy)
	case "container":
		resources = s.convertContainers(snapshot.Containers, limit, sortBy)
	case "vm":
		resources = s.convertVMs(snapshot.VirtualMachines, limit, sortBy)
	case "pod":
		resources = s.convertPods(snapshot.Pods, limit, sortBy)
	default:
		return nil, fmt.Errorf("unsupported resource type: %s", params.Arguments.ResourceType)
	}

	// Format response
	result := formatTopConsumersResult(resources, params.Arguments.ResourceType, limit)

	return &mcp.CallToolResultFor[any]{
		Content: []mcp.Content{&mcp.TextContent{Text: result}},
	}, nil
}

// handleGetResourcePower handles the get_resource_power tool call
func (s *Server) handleGetResourcePower(ctx context.Context, cc *mcp.ServerSession, params *mcp.CallToolParamsFor[GetResourcePowerParams]) (*mcp.CallToolResultFor[any], error) {
	s.logger.Debug("Handling get_resource_power request",
		"resource_type", params.Arguments.ResourceType,
		"resource_id", params.Arguments.ResourceID)

	snapshot, err := s.monitor.Snapshot()
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}

	var resource *PowerResourceInfo
	switch params.Arguments.ResourceType {
	case "process":
		if process, exists := snapshot.Processes[params.Arguments.ResourceID]; exists {
			converted := s.convertProcesses(map[string]*monitor.Process{params.Arguments.ResourceID: process}, 1, "power")
			if len(converted) > 0 {
				resource = &converted[0]
			}
		}
	case "container":
		if container, exists := snapshot.Containers[params.Arguments.ResourceID]; exists {
			converted := s.convertContainers(map[string]*monitor.Container{params.Arguments.ResourceID: container}, 1, "power")
			if len(converted) > 0 {
				resource = &converted[0]
			}
		}
	case "vm":
		if vm, exists := snapshot.VirtualMachines[params.Arguments.ResourceID]; exists {
			converted := s.convertVMs(map[string]*monitor.VirtualMachine{params.Arguments.ResourceID: vm}, 1, "power")
			if len(converted) > 0 {
				resource = &converted[0]
			}
		}
	case "pod":
		if pod, exists := snapshot.Pods[params.Arguments.ResourceID]; exists {
			converted := s.convertPods(map[string]*monitor.Pod{params.Arguments.ResourceID: pod}, 1, "power")
			if len(converted) > 0 {
				resource = &converted[0]
			}
		}
	default:
		return nil, fmt.Errorf("unsupported resource type: %s", params.Arguments.ResourceType)
	}

	if resource == nil {
		return nil, fmt.Errorf("resource not found: %s/%s", params.Arguments.ResourceType, params.Arguments.ResourceID)
	}

	result := formatResourceDetails(*resource)

	return &mcp.CallToolResultFor[any]{
		Content: []mcp.Content{&mcp.TextContent{Text: result}},
	}, nil
}

// handleSearchResources handles the search_resources tool call
func (s *Server) handleSearchResources(ctx context.Context, cc *mcp.ServerSession, params *mcp.CallToolParamsFor[SearchResourcesParams]) (*mcp.CallToolResultFor[any], error) {
	s.logger.Debug("Handling search_resources request",
		"resource_type", params.Arguments.ResourceType,
		"power_min", params.Arguments.PowerMin,
		"power_max", params.Arguments.PowerMax,
		"name_pattern", params.Arguments.NamePattern)

	snapshot, err := s.monitor.Snapshot()
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}

	limit := params.Arguments.Limit
	if limit <= 0 {
		limit = 10
	}

	var allResources []PowerResourceInfo
	switch params.Arguments.ResourceType {
	case "process":
		allResources = s.convertProcesses(snapshot.Processes, 0, "power") // 0 = no limit initially
	case "container":
		allResources = s.convertContainers(snapshot.Containers, 0, "power")
	case "vm":
		allResources = s.convertVMs(snapshot.VirtualMachines, 0, "power")
	case "pod":
		allResources = s.convertPods(snapshot.Pods, 0, "power")
	default:
		return nil, fmt.Errorf("unsupported resource type: %s", params.Arguments.ResourceType)
	}

	// Apply filters
	filtered := s.filterResources(allResources, params.Arguments)
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}

	result := formatSearchResults(filtered, params.Arguments)

	return &mcp.CallToolResultFor[any]{
		Content: []mcp.Content{&mcp.TextContent{Text: result}},
	}, nil
}

// Helper methods for data conversion and formatting

func (s *Server) convertNode(node *monitor.Node) []PowerResourceInfo {
	if node == nil {
		return []PowerResourceInfo{}
	}

	power := make(map[string]float64)
	energy := make(map[string]float64)

	for zone, usage := range node.Zones {
		power[zone.Name()] = float64(usage.Power)
		energy[zone.Name()] = float64(usage.EnergyTotal)
	}

	return []PowerResourceInfo{{
		Type:        "node",
		ID:          "node",
		Name:        "Node",
		Power:       power,
		EnergyTotal: energy,
		Metadata: map[string]string{
			"usage_ratio": fmt.Sprintf("%.2f", node.UsageRatio),
			"timestamp":   node.Timestamp.Format("2006-01-02T15:04:05Z"),
		},
	}}
}

func (s *Server) convertProcesses(processes map[string]*monitor.Process, limit int, sortBy string) []PowerResourceInfo {
	resources := make([]PowerResourceInfo, 0, len(processes))

	for _, process := range processes {
		power := make(map[string]float64)
		energy := make(map[string]float64)

		for zone, usage := range process.Zones {
			power[zone.Name()] = float64(usage.Power)
			energy[zone.Name()] = float64(usage.EnergyTotal)
		}

		resources = append(resources, PowerResourceInfo{
			Type:        "process",
			ID:          strconv.Itoa(process.PID),
			Name:        process.Comm,
			Power:       power,
			EnergyTotal: energy,
			Metadata: map[string]string{
				"exe":            process.Exe,
				"type":           string(process.Type),
				"cpu_total_time": fmt.Sprintf("%.2f", process.CPUTotalTime),
				"container_id":   process.ContainerID,
				"vm_id":          process.VirtualMachineID,
			},
		})
	}

	return s.sortAndLimit(resources, limit, sortBy)
}

func (s *Server) convertContainers(containers map[string]*monitor.Container, limit int, sortBy string) []PowerResourceInfo {
	resources := make([]PowerResourceInfo, 0, len(containers))

	for _, container := range containers {
		power := make(map[string]float64)
		energy := make(map[string]float64)

		for zone, usage := range container.Zones {
			power[zone.Name()] = float64(usage.Power)
			energy[zone.Name()] = float64(usage.EnergyTotal)
		}

		resources = append(resources, PowerResourceInfo{
			Type:        "container",
			ID:          container.ID,
			Name:        container.Name,
			Power:       power,
			EnergyTotal: energy,
			Metadata: map[string]string{
				"runtime":        string(container.Runtime),
				"cpu_total_time": fmt.Sprintf("%.2f", container.CPUTotalTime),
				"pod_id":         container.PodID,
			},
		})
	}

	return s.sortAndLimit(resources, limit, sortBy)
}

func (s *Server) convertVMs(vms map[string]*monitor.VirtualMachine, limit int, sortBy string) []PowerResourceInfo {
	resources := make([]PowerResourceInfo, 0, len(vms))

	for _, vm := range vms {
		power := make(map[string]float64)
		energy := make(map[string]float64)

		for zone, usage := range vm.Zones {
			power[zone.Name()] = float64(usage.Power)
			energy[zone.Name()] = float64(usage.EnergyTotal)
		}

		resources = append(resources, PowerResourceInfo{
			Type:        "vm",
			ID:          vm.ID,
			Name:        vm.Name,
			Power:       power,
			EnergyTotal: energy,
			Metadata: map[string]string{
				"hypervisor":     string(vm.Hypervisor),
				"cpu_total_time": fmt.Sprintf("%.2f", vm.CPUTotalTime),
			},
		})
	}

	return s.sortAndLimit(resources, limit, sortBy)
}

func (s *Server) convertPods(pods map[string]*monitor.Pod, limit int, sortBy string) []PowerResourceInfo {
	resources := make([]PowerResourceInfo, 0, len(pods))

	for _, pod := range pods {
		power := make(map[string]float64)
		energy := make(map[string]float64)

		for zone, usage := range pod.Zones {
			power[zone.Name()] = float64(usage.Power)
			energy[zone.Name()] = float64(usage.EnergyTotal)
		}

		resources = append(resources, PowerResourceInfo{
			Type:        "pod",
			ID:          pod.ID,
			Name:        pod.Name,
			Power:       power,
			EnergyTotal: energy,
			Metadata: map[string]string{
				"namespace":      pod.Namespace,
				"cpu_total_time": fmt.Sprintf("%.2f", pod.CPUTotalTime),
			},
		})
	}

	return s.sortAndLimit(resources, limit, sortBy)
}

func (s *Server) sortAndLimit(resources []PowerResourceInfo, limit int, sortBy string) []PowerResourceInfo {
	// Sort by total power/energy across all zones
	sort.Slice(resources, func(i, j int) bool {
		var valueI, valueJ float64

		if sortBy == "energy" {
			for _, v := range resources[i].EnergyTotal {
				valueI += v
			}
			for _, v := range resources[j].EnergyTotal {
				valueJ += v
			}
		} else {
			for _, v := range resources[i].Power {
				valueI += v
			}
			for _, v := range resources[j].Power {
				valueJ += v
			}
		}

		return valueI > valueJ // Descending order
	})

	if limit > 0 && len(resources) > limit {
		resources = resources[:limit]
	}

	return resources
}

func (s *Server) filterResources(resources []PowerResourceInfo, params SearchResourcesParams) []PowerResourceInfo {
	filtered := make([]PowerResourceInfo, 0)

	for _, resource := range resources {
		// Calculate total power
		totalPower := 0.0
		for _, power := range resource.Power {
			totalPower += power
		}

		// Apply power filters
		if params.PowerMin > 0 && totalPower < params.PowerMin {
			continue
		}
		if params.PowerMax > 0 && totalPower > params.PowerMax {
			continue
		}

		// Apply name pattern filter
		if params.NamePattern != "" && !strings.Contains(strings.ToLower(resource.Name), strings.ToLower(params.NamePattern)) {
			continue
		}

		filtered = append(filtered, resource)
	}

	return filtered
}

// Formatting helper functions

func formatTopConsumersResult(resources []PowerResourceInfo, resourceType string, limit int) string {
	if len(resources) == 0 {
		return fmt.Sprintf("No %s resources found with power consumption data.", resourceType)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Top %d %s consumers:\n\n", len(resources), resourceType))

	for i, resource := range resources {
		totalPower := 0.0
		totalEnergy := 0.0

		for _, power := range resource.Power {
			totalPower += power
		}
		for _, energy := range resource.EnergyTotal {
			totalEnergy += energy
		}

		sb.WriteString(fmt.Sprintf("%d. %s: %s, Name: %s, Power: %.2fW, Energy: %.0fJ\n",
			i+1, resource.Type, resource.ID, resource.Name, totalPower, totalEnergy))
	}

	return sb.String()
}

func formatResourceDetails(resource PowerResourceInfo) string {
	var sb strings.Builder

	totalPower := 0.0
	totalEnergy := 0.0

	for _, power := range resource.Power {
		totalPower += power
	}
	for _, energy := range resource.EnergyTotal {
		totalEnergy += energy
	}

	sb.WriteString(fmt.Sprintf("%s Details:\n", resource.Type))
	sb.WriteString(fmt.Sprintf("ID: %s\n", resource.ID))
	sb.WriteString(fmt.Sprintf("Name: %s\n", resource.Name))
	sb.WriteString(fmt.Sprintf("Total Power: %.2fW\n", totalPower))
	sb.WriteString(fmt.Sprintf("Total Energy: %.0fJ\n", totalEnergy))

	if len(resource.Power) > 1 {
		sb.WriteString("\nPower by Zone:\n")
		for zone, power := range resource.Power {
			sb.WriteString(fmt.Sprintf("  %s: %.2fW\n", zone, power))
		}
	}

	if len(resource.Metadata) > 0 {
		sb.WriteString("\nMetadata:\n")
		for key, value := range resource.Metadata {
			if value != "" {
				sb.WriteString(fmt.Sprintf("  %s: %s\n", key, value))
			}
		}
	}

	return sb.String()
}

func formatSearchResults(resources []PowerResourceInfo, params SearchResourcesParams) string {
	if len(resources) == 0 {
		return fmt.Sprintf("No %s resources found matching the search criteria.", params.ResourceType)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d %s resources matching criteria:\n\n", len(resources), params.ResourceType))

	for i, resource := range resources {
		totalPower := 0.0
		for _, power := range resource.Power {
			totalPower += power
		}

		sb.WriteString(fmt.Sprintf("%d. %s: %s, Power: %.2fW\n",
			i+1, resource.ID, resource.Name, totalPower))
	}

	return sb.String()
}
