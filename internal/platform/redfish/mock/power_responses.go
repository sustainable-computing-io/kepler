// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package mock

// NOTE: all static fixtures should be placed  in testdata/fixtures/

// PowerResponse returns a power response structure
func PowerResponse(powerWatts float64) map[string]any {
	baseResponse := map[string]any{
		"@odata.context": "/redfish/v1/$metadata#Power.Power",
		"@odata.type":    "#Power.v1_5_0.Power",
		"@odata.id":      "/redfish/v1/Chassis/1/Power",
		"Id":             "Power",
		"Name":           "Power",
		"PowerControl": []map[string]any{
			{
				"@odata.id":           "/redfish/v1/Chassis/1/Power#/PowerControl/0",
				"MemberId":            "0",
				"Name":                "System Power Control",
				"PowerConsumedWatts":  powerWatts,
				"PowerRequestedWatts": powerWatts,
				"PowerAvailableWatts": 600.0,
				"PowerCapacityWatts":  750.0,
				"PowerMetrics": map[string]any{
					"IntervalInMin":        1,
					"MinConsumedWatts":     powerWatts * 0.8,
					"MaxConsumedWatts":     powerWatts * 1.2,
					"AverageConsumedWatts": powerWatts,
				},
				"PowerLimit": map[string]any{
					"LimitInWatts":   500.0,
					"LimitException": "NoAction",
				},
			},
		},
	}

	return baseResponse
}
