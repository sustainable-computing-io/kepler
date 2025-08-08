// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package testdata

// PowerResponseFixtures contains JSON fixtures for different power response scenarios
var PowerResponseFixtures = map[string]string{
	"service_root": `{
		"@odata.context": "/redfish/v1/$metadata#ServiceRoot.ServiceRoot",
		"@odata.type": "#ServiceRoot.v1_5_0.ServiceRoot",
		"@odata.id": "/redfish/v1/",
		"Id": "RootService",
		"Name": "Root Service",
		"RedfishVersion": "1.6.1",
		"UUID": "12345678-1234-1234-1234-123456789012",
		"Chassis": {
			"@odata.id": "/redfish/v1/Chassis"
		}
	}`,

	"chassis_collection": `{
		"@odata.context": "/redfish/v1/$metadata#ChassisCollection.ChassisCollection",
		"@odata.type": "#ChassisCollection.ChassisCollection",
		"@odata.id": "/redfish/v1/Chassis",
		"Name": "Chassis Collection",
		"Members@odata.count": 1,
		"Members": [
			{
				"@odata.id": "/redfish/v1/Chassis/1"
			}
		]
	}`,

	"chassis": `{
		"@odata.context": "/redfish/v1/$metadata#Chassis.Chassis",
		"@odata.type": "#Chassis.v1_10_0.Chassis",
		"@odata.id": "/redfish/v1/Chassis/1",
		"Id": "1",
		"Name": "Computer System Chassis",
		"ChassisType": "RackMount",
		"PowerState": "On",
		"Power": {
			"@odata.id": "/redfish/v1/Chassis/1/Power"
		}
	}`,

	"dell_power_245w": `{
		"@odata.context": "/redfish/v1/$metadata#Power.Power",
		"@odata.type": "#Power.v1_5_0.Power",
		"@odata.id": "/redfish/v1/Chassis/System.Embedded.1/Power",
		"Id": "Power",
		"Name": "Power",
		"PowerControl": [
			{
				"@odata.id": "/redfish/v1/Chassis/System.Embedded.1/Power#/PowerControl/0",
				"Name": "System Power Control",
				"PowerConsumedWatts": 245.0,
				"PowerRequestedWatts": 295.0,
				"PowerCapacityWatts": 750.0,
				"PowerMetrics": {
					"IntervalInMin": 60,
					"MinConsumedWatts": 235.0,
					"MaxConsumedWatts": 265.0,
					"AverageConsumedWatts": 250.0
				}
			}
		]
	}`,

	"hpe_power_189w": `{
		"@odata.context": "/redfish/v1/$metadata#Power.Power",
		"@odata.type": "#Power.v1_5_0.Power",
		"@odata.id": "/redfish/v1/Chassis/1/Power",
		"Id": "Power",
		"Name": "Power",
		"PowerControl": [
			{
				"@odata.id": "/redfish/v1/Chassis/1/Power#/PowerControl/0",
				"Name": "Server Power Control",
				"PowerConsumedWatts": 189.5,
				"PowerRequestedWatts": 214.5,
				"PowerAvailableWatts": 800.0,
				"PowerAllocatedWatts": 289.5
			}
		]
	}`,

	"lenovo_power_167w": `{
		"@odata.context": "/redfish/v1/$metadata#Power.Power",
		"@odata.type": "#Power.v1_5_0.Power",
		"@odata.id": "/redfish/v1/Chassis/1/Power",
		"Id": "Power",
		"Name": "Power",
		"PowerControl": [
			{
				"@odata.id": "/redfish/v1/Chassis/1/Power#/PowerControl/0",
				"Name": "Node Power Control",
				"PowerConsumedWatts": 167.8,
				"PowerCapacityWatts": 550.0
			}
		]
	}`,

	"generic_power_200w": `{
		"@odata.context": "/redfish/v1/$metadata#Power.Power",
		"@odata.type": "#Power.v1_5_0.Power",
		"@odata.id": "/redfish/v1/Chassis/System/Power",
		"Id": "Power",
		"Name": "Power",
		"PowerControl": [
			{
				"@odata.id": "/redfish/v1/Chassis/System/Power#/PowerControl/0",
				"Name": "System Power Control",
				"PowerConsumedWatts": 200.0,
				"PowerCapacityWatts": 650.0
			}
		]
	}`,

	"zero_power": `{
		"@odata.context": "/redfish/v1/$metadata#Power.Power",
		"@odata.type": "#Power.v1_5_0.Power",
		"@odata.id": "/redfish/v1/Chassis/1/Power",
		"Id": "Power",
		"Name": "Power",
		"PowerControl": [
			{
				"@odata.id": "/redfish/v1/Chassis/1/Power#/PowerControl/0",
				"Name": "System Power Control",
				"PowerConsumedWatts": 0.0,
				"PowerCapacityWatts": 650.0
			}
		]
	}`,

	"empty_power_control": `{
		"@odata.context": "/redfish/v1/$metadata#Power.Power",
		"@odata.type": "#Power.v1_5_0.Power",
		"@odata.id": "/redfish/v1/Chassis/1/Power",
		"Id": "Power",
		"Name": "Power",
		"PowerControl": []
	}`,

	"error_not_found": `{
		"error": {
			"code": "Base.1.0.ResourceNotFound",
			"message": "The requested resource was not found.",
			"@Message.ExtendedInfo": [
				{
					"MessageId": "Base.1.0.ResourceNotFound",
					"Message": "The requested resource of type Power was not found.",
					"Severity": "Critical",
					"Resolution": "Check the URI and resubmit the request."
				}
			]
		}
	}`,

	"error_auth_failed": `{
		"error": {
			"code": "Base.1.0.GeneralError",
			"message": "Authentication failed",
			"@Message.ExtendedInfo": [
				{
					"MessageId": "Base.1.0.SessionLimitExceeded",
					"Message": "The session establishment failed due to authentication failure.",
					"Severity": "Critical",
					"Resolution": "Log in with proper credentials."
				}
			]
		}
	}`,
}

// GetFixture returns a fixture by name, panics if not found (for tests)
func GetFixture(name string) string {
	fixture, exists := PowerResponseFixtures[name]
	if !exists {
		panic("fixture not found: " + name)
	}
	return fixture
}
