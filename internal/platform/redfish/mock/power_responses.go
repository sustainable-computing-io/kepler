// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package mock

// GetPowerResponse returns vendor-specific power response structures
func GetPowerResponse(vendor VendorType, powerWatts float64) map[string]interface{} {
	baseResponse := map[string]interface{}{
		"@odata.context": "/redfish/v1/$metadata#Power.Power",
		"@odata.type":    "#Power.v1_5_0.Power",
		"@odata.id":      "/redfish/v1/Chassis/1/Power",
		"Id":             "Power",
		"Name":           "Power",
	}

	switch vendor {
	case VendorDell:
		return getDellPowerResponse(baseResponse, powerWatts)
	case VendorHPE:
		return getHPEPowerResponse(baseResponse, powerWatts)
	case VendorLenovo:
		return getLenovoPowerResponse(baseResponse, powerWatts)
	default:
		return getGenericPowerResponse(baseResponse, powerWatts)
	}
}

func getDellPowerResponse(base map[string]interface{}, powerWatts float64) map[string]interface{} {
	base["PowerControl"] = []map[string]interface{}{
		{
			"@odata.id":           "/redfish/v1/Chassis/1/Power#/PowerControl/0",
			"MemberId":            "0",
			"Name":                "System Power Control",
			"PowerConsumedWatts":  powerWatts,
			"PowerRequestedWatts": powerWatts * 1.1,
			"PowerAvailableWatts": 650.0,
			"PowerCapacityWatts":  750.0,
			"PowerAllocatedWatts": powerWatts,
			"PowerMetrics": map[string]interface{}{
				"IntervalInMin":        1,
				"MinConsumedWatts":     powerWatts * 0.8,
				"MaxConsumedWatts":     powerWatts * 1.2,
				"AverageConsumedWatts": powerWatts,
			},
			"PowerLimit": map[string]interface{}{
				"LimitInWatts":   500.0,
				"LimitException": "NoAction",
			},
			"RelatedItem": []map[string]interface{}{
				{
					"@odata.id": "/redfish/v1/Chassis/1",
				},
			},
		},
	}
	return base
}

func getHPEPowerResponse(base map[string]interface{}, powerWatts float64) map[string]interface{} {
	base["PowerControl"] = []map[string]interface{}{
		{
			"@odata.id":           "/redfish/v1/Chassis/1/Power#/PowerControl/0",
			"MemberId":            "0",
			"Name":                "Server Power Control",
			"PowerConsumedWatts":  powerWatts,
			"PowerRequestedWatts": powerWatts,
			"PowerAvailableWatts": 800.0,
			"PowerCapacityWatts":  1000.0,
			"PowerMetrics": map[string]interface{}{
				"IntervalInMin":        1,
				"MinConsumedWatts":     powerWatts * 0.85,
				"MaxConsumedWatts":     powerWatts * 1.15,
				"AverageConsumedWatts": powerWatts,
			},
			"PowerLimit": map[string]interface{}{
				"LimitInWatts":   600.0,
				"LimitException": "HardPowerOff",
			},
		},
	}
	return base
}

func getLenovoPowerResponse(base map[string]interface{}, powerWatts float64) map[string]interface{} {
	base["PowerControl"] = []map[string]interface{}{
		{
			"@odata.id":           "/redfish/v1/Chassis/1/Power#/PowerControl/0",
			"MemberId":            "0",
			"Name":                "Chassis Power Control",
			"PowerConsumedWatts":  powerWatts,
			"PowerRequestedWatts": powerWatts,
			"PowerAvailableWatts": 550.0,
			"PowerCapacityWatts":  600.0,
			"PowerMetrics": map[string]interface{}{
				"IntervalInMin":        5,
				"MinConsumedWatts":     powerWatts * 0.9,
				"MaxConsumedWatts":     powerWatts * 1.1,
				"AverageConsumedWatts": powerWatts,
			},
			"PowerLimit": map[string]interface{}{
				"LimitInWatts":   450.0,
				"LimitException": "LogEventOnly",
			},
		},
	}
	return base
}

func getGenericPowerResponse(base map[string]interface{}, powerWatts float64) map[string]interface{} {
	base["PowerControl"] = []map[string]interface{}{
		{
			"@odata.id":           "/redfish/v1/Chassis/1/Power#/PowerControl/0",
			"MemberId":            "0",
			"Name":                "System Power Control",
			"PowerConsumedWatts":  powerWatts,
			"PowerRequestedWatts": powerWatts,
			"PowerAvailableWatts": 500.0,
			"PowerCapacityWatts":  600.0,
			"PowerMetrics": map[string]interface{}{
				"IntervalInMin":        1,
				"MinConsumedWatts":     powerWatts * 0.8,
				"MaxConsumedWatts":     powerWatts * 1.2,
				"AverageConsumedWatts": powerWatts,
			},
			"PowerLimit": map[string]interface{}{
				"LimitInWatts":   400.0,
				"LimitException": "NoAction",
			},
		},
	}
	return base
}
