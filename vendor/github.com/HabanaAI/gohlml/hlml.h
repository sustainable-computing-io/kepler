/* SPDX-License-Identifier: MIT
 *
 * Copyright 2016-2019 HabanaLabs, Ltd.
 * All Rights Reserved.
 *
 */

#ifndef __HLML_H__
#define __HLML_H__

#include <net/ethernet.h>
#include <stdbool.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

#define PCI_DOMAIN_LEN		5
#define PCI_ADDR_LEN		((PCI_DOMAIN_LEN) + 10)
#define PCI_LINK_INFO_LEN	10
#define HL_FIELD_MAX_SIZE	32

#define PORTS_ARR_SIZE	2

#define HLML_DEVICE_MAC_MAX_ADDRESSES	48

#define HLML_DEVICE_ROW_RPL_MAX		32

/*
 * Event HLML_EVENT_ECC_ERR is used with UNCORRECTABLE (DERR) only,
 * i.e. only for DERR events, therefore, it is remained with this
 * name only for backward compatability, although better use
 * HLML_EVENT_ECC_DERR instead.
 */
#define HLML_EVENT_ECC_ERR		(1 << 0)
/* Event about critical errors that occurred on the device */
#define HLML_EVENT_CRITICAL_ERR		(1 << 1)
/* Event about changes in clock rate */
#define HLML_EVENT_CLOCK_RATE		(1 << 2)
/* Memory location DRAM */
#define HLML_EVENT_DRAM_ERR		(1 << 3)
/* Event about double bit ECC errors. */
#define HLML_EVENT_ECC_DERR		(1 << 0)
/* Event about single bit ECC errors. */
#define HLML_EVENT_ECC_SERR		(1 << 4)

/* Bit masks representing all supported clocks throttling reasons */
#define HLML_CLOCKS_THROTTLE_REASON_POWER	(1 << 0)
#define HLML_CLOCKS_THROTTLE_REASON_THERMAL	(1 << 1)

/* Scope of NUMA node for affinity queries */
#define HLML_AFFINITY_SCOPE_NODE     0
/* Scope of processor socket for affinity queries */
#define HLML_AFFINITY_SCOPE_SOCKET   1

typedef unsigned int hlml_affinity_scope_t;

/* Enum for returned values of the different APIs */
typedef enum hlml_return {
	HLML_SUCCESS = 0,
	HLML_ERROR_UNINITIALIZED = 1,
	HLML_ERROR_INVALID_ARGUMENT = 2,
	HLML_ERROR_NOT_SUPPORTED = 3,
	HLML_ERROR_ALREADY_INITIALIZED = 5,
	HLML_ERROR_NOT_FOUND = 6,
	HLML_ERROR_INSUFFICIENT_SIZE = 7,
	HLML_ERROR_DRIVER_NOT_LOADED = 9,
	HLML_ERROR_TIMEOUT = 10,
	HLML_ERROR_AIP_IS_LOST = 15,
	HLML_ERROR_MEMORY = 20,
	HLML_ERROR_NO_DATA = 21,
	HLML_ERROR_UNKNOWN = 49,
} hlml_return_t;

/*
 * link_speed - current pci link speed
 * link_width - current pci link width
 */
typedef struct hlml_pci_cap {
	char link_speed[PCI_LINK_INFO_LEN];
	char link_width[PCI_LINK_INFO_LEN];
} hlml_pci_cap_t;

/*
 * bus - The bus on which the device resides, 0 to 0xf
 * bus_id - The tuple domain:bus:device.function
 * device - The device's id on the bus, 0 to 31
 * domain - The PCI domain on which the device's bus resides
 * pci_device_id - The combined 16b deviceId and 16b vendor id
 */
typedef struct hlml_pci_info {
	unsigned int bus;
	char bus_id[PCI_ADDR_LEN];
	unsigned int device;
	unsigned int domain;
	unsigned int pci_device_id;
	hlml_pci_cap_t caps;
} hlml_pci_info_t;

typedef enum hlml_clock_type {
	HLML_CLOCK_SOC = 0,
	HLML_CLOCK_IC = 1,
	HLML_CLOCK_MME = 2,
	HLML_CLOCK_TPC = 3,
	HLML_CLOCK_COUNT
} hlml_clock_type_t;

typedef struct hlml_utilization {
	unsigned int aip;
} hlml_utilization_t;

typedef struct hlml_memory {
	unsigned long long free;
	unsigned long long total; /* Total installed memory (in bytes) */
	unsigned long long used;
} hlml_memory_t;

typedef enum hlml_temperature_sensors {
	HLML_TEMPERATURE_ON_AIP = 0,
	HLML_TEMPERATURE_ON_BOARD = 1,
	HLML_TEMPERATURE_OTHER = 2,
} hlml_temperature_sensors_t;

typedef enum hlml_temperature_thresholds {
	HLML_TEMPERATURE_THRESHOLD_SHUTDOWN = 0,
	HLML_TEMPERATURE_THRESHOLD_SLOWDOWN = 1,
	HLML_TEMPERATURE_THRESHOLD_MEM_MAX = 2,
	HLML_TEMPERATURE_THRESHOLD_GPU_MAX = 3,
	HLML_TEMPERATURE_THRESHOLD_COUNT
} hlml_temperature_thresholds_t;

typedef enum hlml_enable_state {
	HLML_FEATURE_DISABLED = 0,
	HLML_FEATURE_ENABLED = 1
} hlml_enable_state_t;

typedef enum hlml_p_states {
	HLML_PSTATE_0 = 0,
	HLML_PSTATE_UNKNOWN = 32
} hlml_p_states_t;

typedef enum hlml_memory_error_type {
	HLML_MEMORY_ERROR_TYPE_CORRECTED = 0, /* Not supported*/
	HLML_MEMORY_ERROR_TYPE_UNCORRECTED = 1,
	HLML_MEMORY_ERROR_TYPE_COUNT
} hlml_memory_error_type_t;

typedef enum hlml_memory_location_type {
	HLML_MEMORY_LOCATION_SRAM = 0,
	HLML_MEMORY_LOCATION_DRAM = 1,
	HLML_MEMORY_LOCATION_COUNT
} hlml_memory_location_type_t;

typedef enum hlml_ecc_counter_type {
	HLML_VOLATILE_ECC = 0,
	HLML_AGGREGATE_ECC = 1,
	HLML_ECC_COUNTER_TYPE_COUNT
} hlml_ecc_counter_type_t;

typedef enum hlml_err_inject {
	HLML_ERR_INJECT_ENDLESS_COMMAND = 0,
	HLML_ERR_INJECT_NON_FATAL_EVENT = 1,
	HLML_ERR_INJECT_FATAL_EVENT = 2,
	HLML_ERR_INJECT_LOSS_OF_HEARTBEAT = 3,
	HLML_ERR_INJECT_THERMAL_EVENT = 4,
	HLML_ERR_INJECT_COUNT
} hlml_err_inject_t;

/*
 * pcb_ver - The device's PCB version
 * pcb_assembly_ver - The device's PCB Assembly version
 */
typedef struct hlml_pcb_info {
	char pcb_ver[HL_FIELD_MAX_SIZE];
	char pcb_assembly_ver[HL_FIELD_MAX_SIZE];
} hlml_pcb_info_t;

typedef void* hlml_device_t;

typedef struct hlml_event_data {
	hlml_device_t device; /* Specific device where the event occurred. */
	unsigned long long event_type; /* Specific event that occurred */
} hlml_event_data_t;

typedef void* hlml_event_set_t;

typedef struct hlml_mac_info {
	unsigned char addr[ETHER_ADDR_LEN];
	int id;
} hlml_mac_info_t;

typedef struct hlml_nic_stats_info {
	uint32_t port;
	char *str_buf;
	uint64_t *val_buf;
	uint32_t *num_of_counters_out;
} hlml_nic_stats_info_t;

typedef enum hlml_pcie_util_counter {
	HLML_PCIE_UTIL_TX_BYTES = 0,
	HLML_PCIE_UTIL_RX_BYTES = 1,
	HLML_PCIE_UTIL_COUNT,
} hlml_pcie_util_counter_t;

typedef enum hlml_perf_policy_type {
	HLML_PERF_POLICY_POWER = 0,
	HLML_PERF_POLICY_THERMAL = 1,
	HLML_PERF_POLICY_COUNT
} hlml_perf_policy_type_t;

typedef struct hlml_violation_time {
	unsigned long long  reference_time;
	unsigned long long  violation_time;
} hlml_violation_time_t;

typedef enum hlml_row_replacement_cause {
	HLML_ROW_REPLACEMENT_CAUSE_MULTIPLE_SINGLE_BIT_ECC_ERRORS = 0,
	HLML_ROW_REPLACEMENT_CAUSE_DOUBLE_BIT_ECC_ERROR = 1,
	HLML_ROW_REPLACEMENT_CAUSE_COUNT
} hlml_row_replacement_cause_t;

typedef struct hlml_row_address {
	uint8_t hbm_idx;
	uint8_t pc;
	uint8_t sid;
	uint8_t bank_idx;
	uint16_t row_addr;
} hlml_row_address_t;

/* supported APIs */
hlml_return_t hlml_init(void);

hlml_return_t hlml_init_with_flags(unsigned int flags);

hlml_return_t hlml_shutdown(void);

hlml_return_t hlml_device_get_count(unsigned int *device_count);

hlml_return_t hlml_device_get_handle_by_pci_bus_id(const char *pci_addr, hlml_device_t *device);

hlml_return_t hlml_device_get_handle_by_index(unsigned int index, hlml_device_t *device);

hlml_return_t hlml_device_get_handle_by_UUID (const char* uuid, hlml_device_t *device);

hlml_return_t hlml_device_get_name(hlml_device_t device, char *name,
				   unsigned int  length);

hlml_return_t hlml_device_get_pci_info(hlml_device_t device,
				       hlml_pci_info_t *pci);

hlml_return_t hlml_device_get_clock_info(hlml_device_t device,
					 hlml_clock_type_t type,
					 unsigned int *clock);

hlml_return_t hlml_device_get_max_clock_info(hlml_device_t device,
					     hlml_clock_type_t type,
					     unsigned int *clock);

hlml_return_t hlml_device_get_utilization_rates(hlml_device_t device,
					hlml_utilization_t *utilization);

hlml_return_t hlml_device_get_memory_info(hlml_device_t device,
					  hlml_memory_t *memory);

hlml_return_t hlml_device_get_temperature(hlml_device_t device,
					  hlml_temperature_sensors_t sensor_type,
					  unsigned int *temp);

hlml_return_t hlml_device_get_temperature_threshold(hlml_device_t device,
				hlml_temperature_thresholds_t threshold_type,
				unsigned int *temp);

// API is not supported
hlml_return_t hlml_device_get_persistence_mode(hlml_device_t device,
						hlml_enable_state_t *mode);

// API is not supported
hlml_return_t hlml_device_get_performance_state(hlml_device_t device,
						hlml_p_states_t *p_state);

hlml_return_t hlml_device_get_power_usage(hlml_device_t device,
					  unsigned int *power);

hlml_return_t hlml_device_get_power_management_default_limit(hlml_device_t device,
						unsigned int *default_limit);

hlml_return_t hlml_device_get_ecc_mode(hlml_device_t device,
				       hlml_enable_state_t *current,
				       hlml_enable_state_t *pending);

hlml_return_t hlml_device_get_total_ecc_errors(hlml_device_t device,
					hlml_memory_error_type_t error_type,
					hlml_ecc_counter_type_t counter_type,
					unsigned long long *ecc_counts);

hlml_return_t hlml_device_get_memory_error_counter(hlml_device_t device,
					hlml_memory_error_type_t error_type,
					hlml_ecc_counter_type_t counter_type,
					hlml_memory_location_type_t location,
					unsigned long long *ecc_counts);

hlml_return_t hlml_device_get_uuid(hlml_device_t device,
				   char *uuid,
				   unsigned int length);

hlml_return_t hlml_device_get_minor_number(hlml_device_t device,
					   unsigned int *minor_number);

hlml_return_t hlml_device_register_events(hlml_device_t device,
					  unsigned long long event_types,
					  hlml_event_set_t set);

hlml_return_t hlml_event_set_create(hlml_event_set_t *set);

hlml_return_t hlml_event_set_free(hlml_event_set_t set);

hlml_return_t hlml_event_set_wait(hlml_event_set_t set,
				  hlml_event_data_t *data,
				  unsigned int timeoutms);

hlml_return_t hlml_device_get_mac_info(hlml_device_t device,
				       hlml_mac_info_t *mac_info,
				       unsigned int mac_info_size,
				       unsigned int start_mac_id,
				       unsigned int *actual_mac_count);

hlml_return_t hlml_device_err_inject(hlml_device_t device, hlml_err_inject_t err_type);

hlml_return_t hlml_device_get_hl_revision(hlml_device_t device, int *hl_revision);

hlml_return_t hlml_device_get_pcb_info(hlml_device_t device, hlml_pcb_info_t *pcb);

hlml_return_t hlml_device_get_serial(hlml_device_t device, char *serial, unsigned int length);

hlml_return_t hlml_device_get_module_id(hlml_device_t device, unsigned int *module_id);

hlml_return_t hlml_device_get_board_id(hlml_device_t device, unsigned int* board_id);

hlml_return_t hlml_device_get_pcie_throughput(hlml_device_t device,
					      hlml_pcie_util_counter_t counter,
					      unsigned int *value);

hlml_return_t hlml_device_get_pcie_replay_counter(hlml_device_t device, unsigned int *value);

hlml_return_t hlml_device_get_curr_pcie_link_generation(hlml_device_t device,
							unsigned int *curr_link_gen);

hlml_return_t hlml_device_get_curr_pcie_link_width(hlml_device_t device,
						   unsigned int *curr_link_width);

hlml_return_t hlml_device_get_current_clocks_throttle_reasons(hlml_device_t device,
		unsigned long long *clocks_throttle_reasons);

hlml_return_t hlml_device_get_total_energy_consumption(hlml_device_t device,
		unsigned long long *energy);

hlml_return_t hlml_get_mac_addr_info(hlml_device_t device, uint64_t *mask, uint64_t *ext_mask);

hlml_return_t hlml_nic_get_link(hlml_device_t device, uint32_t port, bool *up);

hlml_return_t hlml_nic_get_statistics(hlml_device_t device, hlml_nic_stats_info_t *stats_info);

hlml_return_t hlml_device_clear_cpu_affinity(hlml_device_t device);

hlml_return_t hlml_device_get_cpu_affinity(hlml_device_t device,
					   unsigned int cpu_set_size,
					   unsigned long *cpu_set);

hlml_return_t hlml_device_get_cpu_affinity_within_scope(hlml_device_t device,
							unsigned int cpu_set_size,
							unsigned long *cpu_set,
							hlml_affinity_scope_t scope);

hlml_return_t hlml_device_get_memory_affinity(hlml_device_t device,
					      unsigned int node_set_size,
					      unsigned long *node_set,
					      hlml_affinity_scope_t scope);

hlml_return_t hlml_device_set_cpu_affinity(hlml_device_t device);

hlml_return_t hlml_device_get_violation_status(hlml_device_t device,
					       hlml_perf_policy_type_t perf_policy_type,
					       hlml_violation_time_t *viol_time);

hlml_return_t hlml_device_get_replaced_rows(hlml_device_t device,
					    hlml_row_replacement_cause_t cause,
					    unsigned int *row_count,
					    hlml_row_address_t *addresses);

hlml_return_t hlml_device_get_replaced_rows_pending_status(hlml_device_t device,
							   hlml_enable_state_t *is_pending);

hlml_return_t hlml_get_hlml_version(char *version, unsigned int length);

hlml_return_t hlml_get_driver_version(char *driver_version, unsigned int length);

hlml_return_t hlml_get_model_number(hlml_device_t device, char *model_number,
				    unsigned int length);

hlml_return_t hlml_get_serial_number(hlml_device_t device, char *serial_number,
				     unsigned int length);

hlml_return_t hlml_get_firmware_fit_version(hlml_device_t device, char *firmware_fit,
					    unsigned int length);

hlml_return_t hlml_get_firmware_spi_version(hlml_device_t device, char *firmware_spi,
					    unsigned int length);

hlml_return_t hlml_get_fw_boot_version(hlml_device_t device, char *fw_boot_version,
				       unsigned int length);

hlml_return_t hlml_get_fw_os_version(hlml_device_t device, char *fw_os_version,
				     unsigned int length);
hlml_return_t hlml_get_cpld_version(hlml_device_t device, char *cpld_version,
				    unsigned int length);

#ifdef __cplusplus
}   //extern "C"
#endif

#endif /* __HLML_H__ */
