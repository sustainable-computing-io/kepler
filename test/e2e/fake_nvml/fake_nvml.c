// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

// fake_nvml.c - Fake libnvidia-ml.so.1 for GPU e2e testing
//
// This shared library implements the NVML C API functions that go-nvml
// loads via dlopen/dlsym. It reads device configuration from a JSON file
// specified by the FAKE_NVML_CONFIG environment variable and returns
// canned GPU data. This allows testing Kepler's GPU code paths without
// real NVIDIA hardware.
//
// Build: gcc -shared -fPIC -o libnvidia-ml.so.1 fake_nvml.c cJSON.c -lpthread

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <pthread.h>

#include "cJSON.h"

// ---------- NVML type definitions (matching nvml.h) ----------

typedef int nvmlReturn_t;

#define NVML_SUCCESS                       0
#define NVML_ERROR_UNINITIALIZED           1
#define NVML_ERROR_INVALID_ARGUMENT        2
#define NVML_ERROR_NOT_SUPPORTED           3
#define NVML_ERROR_INSUFFICIENT_SIZE       7
#define NVML_ERROR_NOT_FOUND               6
#define NVML_ERROR_UNKNOWN                 999

typedef void* nvmlDevice_t;

typedef unsigned int nvmlComputeMode_t;
#define NVML_COMPUTEMODE_DEFAULT           0
#define NVML_COMPUTEMODE_EXCLUSIVE_THREAD  1
#define NVML_COMPUTEMODE_PROHIBITED        2
#define NVML_COMPUTEMODE_EXCLUSIVE_PROCESS 3

typedef unsigned int nvmlEnableState_t;
#define NVML_FEATURE_DISABLED  0
#define NVML_FEATURE_ENABLED   1

// v1 process info struct: {pid:u32, pad:4, usedGpuMemory:u64}
typedef struct {
    unsigned int        pid;
    unsigned long long  usedGpuMemory;
} nvmlProcessInfo_v1_t;

// v2 process info struct (also used as nvmlProcessInfo_t for v3)
typedef struct {
    unsigned int        pid;
    unsigned long long  usedGpuMemory;
    unsigned int        gpuInstanceId;
    unsigned int        computeInstanceId;
} nvmlProcessInfo_v2_t;

typedef struct {
    unsigned int        pid;
    unsigned long long  timeStamp;
    unsigned int        smUtil;
    unsigned int        memUtil;
    unsigned int        encUtil;
    unsigned int        decUtil;
} nvmlProcessUtilizationSample_t;

// ---------- Internal state ----------

#define MAX_DEVICES    16
#define MAX_PROCESSES  64
#define MAX_MIG_DEVICES 8

typedef struct {
    unsigned int pid;
    unsigned long long memoryUsed;
    unsigned int smUtil;
} FakeProcess;

typedef struct {
    char uuid[80];
    char name[80];
    unsigned int powerUsageMilliWatts;
    nvmlComputeMode_t computeMode;
    int migEnabled;
    int maxMigDevices;
    FakeProcess processes[MAX_PROCESSES];
    int processCount;
} FakeDevice;

static FakeDevice g_devices[MAX_DEVICES];
static int g_deviceCount = 0;
static int g_initialized = 0;
static struct timespec g_initTime;
static pthread_mutex_t g_mutex = PTHREAD_MUTEX_INITIALIZER;

// ---------- Handle encoding ----------
// Top-level GPU i:   (void*)(uintptr_t)(i + 1)
// MIG device p,m:    (void*)(uintptr_t)((p+1)*100 + m+1)

static inline int handle_to_index(nvmlDevice_t device) {
    unsigned long h = (unsigned long)(device);
    if (h == 0) return -1;
    if (h >= 100) return -1; // MIG device, not a top-level GPU
    return (int)(h - 1);
}

static inline int handle_is_mig(nvmlDevice_t device) {
    unsigned long h = (unsigned long)(device);
    return h >= 100;
}

static inline int handle_to_parent(nvmlDevice_t device) {
    unsigned long h = (unsigned long)(device);
    return (int)(h / 100) - 1;
}

static inline int handle_to_mig_index(nvmlDevice_t device) {
    unsigned long h = (unsigned long)(device);
    return (int)(h % 100) - 1;
}

// ---------- JSON config loading ----------

static int load_config(void) {
    const char *configPath = getenv("FAKE_NVML_CONFIG");
    if (!configPath || *configPath == '\0') {
        fprintf(stderr, "fake_nvml: FAKE_NVML_CONFIG not set\n");
        return -1;
    }

    FILE *f = fopen(configPath, "r");
    if (!f) {
        fprintf(stderr, "fake_nvml: cannot open %s\n", configPath);
        return -1;
    }

    fseek(f, 0, SEEK_END);
    long fsize = ftell(f);
    fseek(f, 0, SEEK_SET);

    char *json_str = malloc(fsize + 1);
    if (!json_str) {
        fclose(f);
        return -1;
    }
    fread(json_str, 1, fsize, f);
    json_str[fsize] = '\0';
    fclose(f);

    cJSON *root = cJSON_Parse(json_str);
    free(json_str);
    if (!root) {
        fprintf(stderr, "fake_nvml: JSON parse error\n");
        return -1;
    }

    cJSON *devices = cJSON_GetObjectItem(root, "devices");
    if (!cJSON_IsArray(devices)) {
        fprintf(stderr, "fake_nvml: 'devices' is not an array\n");
        cJSON_Delete(root);
        return -1;
    }

    g_deviceCount = 0;
    cJSON *dev;
    cJSON_ArrayForEach(dev, devices) {
        if (g_deviceCount >= MAX_DEVICES) break;

        FakeDevice *d = &g_devices[g_deviceCount];
        memset(d, 0, sizeof(FakeDevice));

        cJSON *uuid = cJSON_GetObjectItem(dev, "uuid");
        if (cJSON_IsString(uuid))
            snprintf(d->uuid, sizeof(d->uuid), "%s", uuid->valuestring);
        else
            snprintf(d->uuid, sizeof(d->uuid), "GPU-FAKE-%04d", g_deviceCount);

        cJSON *name = cJSON_GetObjectItem(dev, "name");
        if (cJSON_IsString(name))
            snprintf(d->name, sizeof(d->name), "%s", name->valuestring);
        else
            snprintf(d->name, sizeof(d->name), "NVIDIA Fake GPU %d", g_deviceCount);

        cJSON *power = cJSON_GetObjectItem(dev, "powerUsageMilliWatts");
        d->powerUsageMilliWatts = cJSON_IsNumber(power) ? (unsigned int)power->valuedouble : 40000;

        cJSON *cm = cJSON_GetObjectItem(dev, "computeMode");
        d->computeMode = cJSON_IsNumber(cm) ? (unsigned int)cm->valuedouble : NVML_COMPUTEMODE_DEFAULT;

        cJSON *mig = cJSON_GetObjectItem(dev, "migEnabled");
        d->migEnabled = cJSON_IsTrue(mig) ? 1 : 0;

        cJSON *maxMig = cJSON_GetObjectItem(dev, "maxMigDevices");
        d->maxMigDevices = cJSON_IsNumber(maxMig) ? (int)maxMig->valuedouble : 0;

        cJSON *procs = cJSON_GetObjectItem(dev, "processes");
        d->processCount = 0;
        if (cJSON_IsArray(procs)) {
            cJSON *proc;
            cJSON_ArrayForEach(proc, procs) {
                if (d->processCount >= MAX_PROCESSES) break;

                FakeProcess *p = &d->processes[d->processCount];
                cJSON *pid = cJSON_GetObjectItem(proc, "pid");
                p->pid = cJSON_IsNumber(pid) ? (unsigned int)pid->valuedouble : 0;

                cJSON *mem = cJSON_GetObjectItem(proc, "memoryUsed");
                p->memoryUsed = cJSON_IsNumber(mem) ? (unsigned long long)mem->valuedouble : 0;

                cJSON *sm = cJSON_GetObjectItem(proc, "smUtil");
                p->smUtil = cJSON_IsNumber(sm) ? (unsigned int)sm->valuedouble : 0;

                d->processCount++;
            }
        }

        g_deviceCount++;
    }

    cJSON_Delete(root);
    fprintf(stderr, "fake_nvml: loaded %d device(s) from %s\n", g_deviceCount, configPath);
    return 0;
}

// ---------- Time helpers ----------

static double elapsed_seconds(void) {
    struct timespec now;
    clock_gettime(CLOCK_MONOTONIC, &now);
    return (double)(now.tv_sec - g_initTime.tv_sec)
         + (double)(now.tv_nsec - g_initTime.tv_nsec) / 1e9;
}

// ---------- NVML API implementations ----------

// nvmlInit_v2 is called by go-nvml after version negotiation
nvmlReturn_t nvmlInit_v2(void) {
    pthread_mutex_lock(&g_mutex);
    if (g_initialized) {
        pthread_mutex_unlock(&g_mutex);
        return NVML_SUCCESS;
    }

    clock_gettime(CLOCK_MONOTONIC, &g_initTime);

    if (load_config() != 0) {
        pthread_mutex_unlock(&g_mutex);
        return NVML_ERROR_UNKNOWN;
    }

    g_initialized = 1;
    pthread_mutex_unlock(&g_mutex);
    return NVML_SUCCESS;
}

// nvmlInit_v1 - fallback, same as v2
nvmlReturn_t nvmlInit_v1(void) {
    return nvmlInit_v2();
}

nvmlReturn_t nvmlInitWithFlags(unsigned int flags) {
    (void)flags;
    return nvmlInit_v2();
}

nvmlReturn_t nvmlShutdown(void) {
    pthread_mutex_lock(&g_mutex);
    g_initialized = 0;
    g_deviceCount = 0;
    pthread_mutex_unlock(&g_mutex);
    return NVML_SUCCESS;
}

const char* nvmlErrorString(nvmlReturn_t result) {
    switch (result) {
        case NVML_SUCCESS:                return "Success";
        case NVML_ERROR_UNINITIALIZED:    return "Uninitialized";
        case NVML_ERROR_INVALID_ARGUMENT: return "Invalid Argument";
        case NVML_ERROR_NOT_SUPPORTED:    return "Not Supported";
        case NVML_ERROR_INSUFFICIENT_SIZE:return "Insufficient Size";
        case NVML_ERROR_NOT_FOUND:        return "Not Found";
        default:                          return "Unknown Error";
    }
}

nvmlReturn_t nvmlSystemGetDriverVersion(char *version, unsigned int length) {
    if (!version || length == 0) return NVML_ERROR_INVALID_ARGUMENT;
    snprintf(version, length, "999.99.99");
    return NVML_SUCCESS;
}

nvmlReturn_t nvmlSystemGetNVMLVersion(char *version, unsigned int length) {
    if (!version || length == 0) return NVML_ERROR_INVALID_ARGUMENT;
    snprintf(version, length, "12.999.999");
    return NVML_SUCCESS;
}

nvmlReturn_t nvmlDeviceGetCount_v2(unsigned int *deviceCount) {
    if (!deviceCount) return NVML_ERROR_INVALID_ARGUMENT;
    if (!g_initialized) return NVML_ERROR_UNINITIALIZED;
    *deviceCount = (unsigned int)g_deviceCount;
    return NVML_SUCCESS;
}

nvmlReturn_t nvmlDeviceGetCount_v1(unsigned int *deviceCount) {
    return nvmlDeviceGetCount_v2(deviceCount);
}

nvmlReturn_t nvmlDeviceGetHandleByIndex_v2(unsigned int index, nvmlDevice_t *device) {
    if (!device) return NVML_ERROR_INVALID_ARGUMENT;
    if (!g_initialized) return NVML_ERROR_UNINITIALIZED;
    if (index >= (unsigned int)g_deviceCount) return NVML_ERROR_INVALID_ARGUMENT;
    *device = (nvmlDevice_t)(unsigned long)(index + 1);
    return NVML_SUCCESS;
}

nvmlReturn_t nvmlDeviceGetHandleByIndex_v1(unsigned int index, nvmlDevice_t *device) {
    return nvmlDeviceGetHandleByIndex_v2(index, device);
}

nvmlReturn_t nvmlDeviceGetUUID(nvmlDevice_t device, char *uuid, unsigned int length) {
    if (!uuid || length == 0) return NVML_ERROR_INVALID_ARGUMENT;
    if (!g_initialized) return NVML_ERROR_UNINITIALIZED;

    int idx = handle_to_index(device);
    if (idx < 0 || idx >= g_deviceCount) return NVML_ERROR_INVALID_ARGUMENT;

    snprintf(uuid, length, "%s", g_devices[idx].uuid);
    return NVML_SUCCESS;
}

nvmlReturn_t nvmlDeviceGetName(nvmlDevice_t device, char *name, unsigned int length) {
    if (!name || length == 0) return NVML_ERROR_INVALID_ARGUMENT;
    if (!g_initialized) return NVML_ERROR_UNINITIALIZED;

    int idx = handle_to_index(device);
    if (idx < 0 || idx >= g_deviceCount) return NVML_ERROR_INVALID_ARGUMENT;

    snprintf(name, length, "%s", g_devices[idx].name);
    return NVML_SUCCESS;
}

nvmlReturn_t nvmlDeviceGetPowerUsage(nvmlDevice_t device, unsigned int *power) {
    if (!power) return NVML_ERROR_INVALID_ARGUMENT;
    if (!g_initialized) return NVML_ERROR_UNINITIALIZED;

    int idx = handle_to_index(device);
    if (idx < 0 || idx >= g_deviceCount) return NVML_ERROR_INVALID_ARGUMENT;

    *power = g_devices[idx].powerUsageMilliWatts;
    return NVML_SUCCESS;
}

nvmlReturn_t nvmlDeviceGetTotalEnergyConsumption(nvmlDevice_t device, unsigned long long *energy) {
    if (!energy) return NVML_ERROR_INVALID_ARGUMENT;
    if (!g_initialized) return NVML_ERROR_UNINITIALIZED;

    int idx = handle_to_index(device);
    if (idx < 0 || idx >= g_deviceCount) return NVML_ERROR_INVALID_ARGUMENT;

    // Energy in millijoules = powerMilliWatts * elapsed_seconds
    // P(mW) * t(s) = E(mJ)  (since mW * s = mJ)
    double elapsed = elapsed_seconds();
    *energy = (unsigned long long)(g_devices[idx].powerUsageMilliWatts * elapsed);
    return NVML_SUCCESS;
}

// v1: nvmlProcessInfo_v1_t {pid, usedGpuMemory}
nvmlReturn_t nvmlDeviceGetComputeRunningProcesses(nvmlDevice_t device,
        unsigned int *infoCount, nvmlProcessInfo_v1_t *infos) {
    if (!infoCount) return NVML_ERROR_INVALID_ARGUMENT;
    if (!g_initialized) return NVML_ERROR_UNINITIALIZED;

    int idx = handle_to_index(device);
    if (idx < 0 || idx >= g_deviceCount) return NVML_ERROR_INVALID_ARGUMENT;

    FakeDevice *d = &g_devices[idx];
    unsigned int needed = (unsigned int)d->processCount;

    if (!infos || *infoCount < needed) {
        *infoCount = needed;
        return (needed == 0) ? NVML_SUCCESS : NVML_ERROR_INSUFFICIENT_SIZE;
    }

    for (int i = 0; i < d->processCount; i++) {
        infos[i].pid = d->processes[i].pid;
        infos[i].usedGpuMemory = d->processes[i].memoryUsed;
    }
    *infoCount = needed;
    return NVML_SUCCESS;
}

// v2: nvmlProcessInfo_v2_t {pid, usedGpuMemory, gpuInstanceId, computeInstanceId}
nvmlReturn_t nvmlDeviceGetComputeRunningProcesses_v2(nvmlDevice_t device,
        unsigned int *infoCount, nvmlProcessInfo_v2_t *infos) {
    if (!infoCount) return NVML_ERROR_INVALID_ARGUMENT;
    if (!g_initialized) return NVML_ERROR_UNINITIALIZED;

    int idx = handle_to_index(device);
    if (idx < 0 || idx >= g_deviceCount) return NVML_ERROR_INVALID_ARGUMENT;

    FakeDevice *d = &g_devices[idx];
    unsigned int needed = (unsigned int)d->processCount;

    if (!infos || *infoCount < needed) {
        *infoCount = needed;
        return (needed == 0) ? NVML_SUCCESS : NVML_ERROR_INSUFFICIENT_SIZE;
    }

    for (int i = 0; i < d->processCount; i++) {
        infos[i].pid = d->processes[i].pid;
        infos[i].usedGpuMemory = d->processes[i].memoryUsed;
        infos[i].gpuInstanceId = 0xFFFFFFFF;
        infos[i].computeInstanceId = 0xFFFFFFFF;
    }
    *infoCount = needed;
    return NVML_SUCCESS;
}

// v3 uses same struct as v2
nvmlReturn_t nvmlDeviceGetComputeRunningProcesses_v3(nvmlDevice_t device,
        unsigned int *infoCount, nvmlProcessInfo_v2_t *infos) {
    return nvmlDeviceGetComputeRunningProcesses_v2(device, infoCount, infos);
}

// GetProcessUtilization: go-nvml calls twice:
//   1st: utilization=NULL → set *count, return ERROR_INSUFFICIENT_SIZE
//   2nd: utilization=buffer → fill buffer, return SUCCESS
nvmlReturn_t nvmlDeviceGetProcessUtilization(nvmlDevice_t device,
        nvmlProcessUtilizationSample_t *utilization,
        unsigned int *processSamplesCount,
        unsigned long long lastSeenTimeStamp) {
    (void)lastSeenTimeStamp;
    if (!processSamplesCount) return NVML_ERROR_INVALID_ARGUMENT;
    if (!g_initialized) return NVML_ERROR_UNINITIALIZED;

    int idx = handle_to_index(device);
    if (idx < 0 || idx >= g_deviceCount) return NVML_ERROR_INVALID_ARGUMENT;

    FakeDevice *d = &g_devices[idx];
    unsigned int needed = (unsigned int)d->processCount;

    if (needed == 0) {
        *processSamplesCount = 0;
        return NVML_ERROR_NOT_FOUND;
    }

    // First call: utilization is NULL, return count via ERROR_INSUFFICIENT_SIZE
    if (!utilization) {
        *processSamplesCount = needed;
        return NVML_ERROR_INSUFFICIENT_SIZE;
    }

    // Second call: fill the buffer
    struct timespec now;
    clock_gettime(CLOCK_MONOTONIC, &now);
    unsigned long long ts = (unsigned long long)now.tv_sec * 1000000ULL
                          + (unsigned long long)now.tv_nsec / 1000ULL;

    unsigned int count = *processSamplesCount < needed ? *processSamplesCount : needed;
    for (unsigned int i = 0; i < count; i++) {
        utilization[i].pid = d->processes[i].pid;
        utilization[i].timeStamp = ts;
        utilization[i].smUtil = d->processes[i].smUtil;
        utilization[i].memUtil = 0;
        utilization[i].encUtil = 0;
        utilization[i].decUtil = 0;
    }
    *processSamplesCount = count;
    return NVML_SUCCESS;
}

nvmlReturn_t nvmlDeviceGetComputeMode(nvmlDevice_t device, nvmlComputeMode_t *mode) {
    if (!mode) return NVML_ERROR_INVALID_ARGUMENT;
    if (!g_initialized) return NVML_ERROR_UNINITIALIZED;

    int idx = handle_to_index(device);
    if (idx < 0 || idx >= g_deviceCount) return NVML_ERROR_INVALID_ARGUMENT;

    *mode = g_devices[idx].computeMode;
    return NVML_SUCCESS;
}

nvmlReturn_t nvmlDeviceGetMigMode(nvmlDevice_t device, unsigned int *currentMode, unsigned int *pendingMode) {
    if (!currentMode || !pendingMode) return NVML_ERROR_INVALID_ARGUMENT;
    if (!g_initialized) return NVML_ERROR_UNINITIALIZED;

    int idx = handle_to_index(device);
    if (idx < 0 || idx >= g_deviceCount) return NVML_ERROR_INVALID_ARGUMENT;

    if (!g_devices[idx].migEnabled && g_devices[idx].maxMigDevices == 0) {
        return NVML_ERROR_NOT_SUPPORTED;
    }

    *currentMode = g_devices[idx].migEnabled ? 1 : 0;
    *pendingMode = *currentMode;
    return NVML_SUCCESS;
}

nvmlReturn_t nvmlDeviceGetMigDeviceHandleByIndex(nvmlDevice_t device,
        unsigned int index, nvmlDevice_t *migDevice) {
    if (!migDevice) return NVML_ERROR_INVALID_ARGUMENT;
    if (!g_initialized) return NVML_ERROR_UNINITIALIZED;

    int parentIdx = handle_to_index(device);
    if (parentIdx < 0 || parentIdx >= g_deviceCount) return NVML_ERROR_INVALID_ARGUMENT;

    FakeDevice *d = &g_devices[parentIdx];
    if (!d->migEnabled || (int)index >= d->maxMigDevices) {
        return NVML_ERROR_NOT_FOUND;
    }

    // Encode as MIG handle: (parent+1)*100 + mig+1
    *migDevice = (nvmlDevice_t)(unsigned long)((parentIdx + 1) * 100 + index + 1);
    return NVML_SUCCESS;
}

nvmlReturn_t nvmlDeviceGetGpuInstanceId(nvmlDevice_t device, unsigned int *id) {
    if (!id) return NVML_ERROR_INVALID_ARGUMENT;
    if (!g_initialized) return NVML_ERROR_UNINITIALIZED;

    if (!handle_is_mig(device)) {
        return NVML_ERROR_NOT_SUPPORTED;
    }

    *id = (unsigned int)handle_to_mig_index(device);
    return NVML_SUCCESS;
}

nvmlReturn_t nvmlDeviceGetMaxMigDeviceCount(nvmlDevice_t device, unsigned int *count) {
    if (!count) return NVML_ERROR_INVALID_ARGUMENT;
    if (!g_initialized) return NVML_ERROR_UNINITIALIZED;

    int idx = handle_to_index(device);
    if (idx < 0 || idx >= g_deviceCount) {
        // Might be a MIG device itself
        if (handle_is_mig(device)) {
            *count = 0;
            return NVML_SUCCESS;
        }
        return NVML_ERROR_INVALID_ARGUMENT;
    }

    *count = (unsigned int)g_devices[idx].maxMigDevices;
    return NVML_SUCCESS;
}

nvmlReturn_t nvmlDeviceGetAccountingMode(nvmlDevice_t device, nvmlEnableState_t *mode) {
    (void)device;
    if (!mode) return NVML_ERROR_INVALID_ARGUMENT;
    *mode = NVML_FEATURE_ENABLED;
    return NVML_SUCCESS;
}

// ---------- Stub functions for symbols go-nvml probes ----------
// These are looked up via dlsym for version negotiation but may not
// be actively used. Returning NOT_SUPPORTED is safe.

nvmlReturn_t nvmlDeviceGetPciInfo_v2(nvmlDevice_t device, void *pci) {
    (void)device; (void)pci;
    return NVML_ERROR_NOT_SUPPORTED;
}

nvmlReturn_t nvmlDeviceGetPciInfo_v3(nvmlDevice_t device, void *pci) {
    (void)device; (void)pci;
    return NVML_ERROR_NOT_SUPPORTED;
}

nvmlReturn_t nvmlDeviceGetNvLinkRemotePciInfo_v2(nvmlDevice_t device,
        unsigned int link, void *pci) {
    (void)device; (void)link; (void)pci;
    return NVML_ERROR_NOT_SUPPORTED;
}

nvmlReturn_t nvmlDeviceGetGridLicensableFeatures_v2(nvmlDevice_t device, void *features) {
    (void)device; (void)features;
    return NVML_ERROR_NOT_SUPPORTED;
}

nvmlReturn_t nvmlDeviceGetGridLicensableFeatures_v3(nvmlDevice_t device, void *features) {
    (void)device; (void)features;
    return NVML_ERROR_NOT_SUPPORTED;
}

nvmlReturn_t nvmlDeviceGetGridLicensableFeatures_v4(nvmlDevice_t device, void *features) {
    (void)device; (void)features;
    return NVML_ERROR_NOT_SUPPORTED;
}

nvmlReturn_t nvmlEventSetWait_v2(void *set, void *data, unsigned int timeoutms) {
    (void)set; (void)data; (void)timeoutms;
    return NVML_ERROR_NOT_SUPPORTED;
}

nvmlReturn_t nvmlDeviceGetAttributes_v2(nvmlDevice_t device, void *attributes) {
    (void)device; (void)attributes;
    return NVML_ERROR_NOT_SUPPORTED;
}

nvmlReturn_t nvmlComputeInstanceGetInfo_v2(void *ci, void *info) {
    (void)ci; (void)info;
    return NVML_ERROR_NOT_SUPPORTED;
}

nvmlReturn_t nvmlDeviceGetGraphicsRunningProcesses(nvmlDevice_t device,
        unsigned int *infoCount, void *infos) {
    (void)device; (void)infos;
    if (infoCount) *infoCount = 0;
    return NVML_SUCCESS;
}

nvmlReturn_t nvmlDeviceGetGraphicsRunningProcesses_v2(nvmlDevice_t device,
        unsigned int *infoCount, void *infos) {
    (void)device; (void)infos;
    if (infoCount) *infoCount = 0;
    return NVML_SUCCESS;
}

nvmlReturn_t nvmlDeviceGetGraphicsRunningProcesses_v3(nvmlDevice_t device,
        unsigned int *infoCount, void *infos) {
    (void)device; (void)infos;
    if (infoCount) *infoCount = 0;
    return NVML_SUCCESS;
}

nvmlReturn_t nvmlDeviceGetMPSComputeRunningProcesses(nvmlDevice_t device,
        unsigned int *infoCount, void *infos) {
    (void)device; (void)infos;
    if (infoCount) *infoCount = 0;
    return NVML_SUCCESS;
}

nvmlReturn_t nvmlDeviceGetMPSComputeRunningProcesses_v2(nvmlDevice_t device,
        unsigned int *infoCount, void *infos) {
    (void)device; (void)infos;
    if (infoCount) *infoCount = 0;
    return NVML_SUCCESS;
}

nvmlReturn_t nvmlDeviceGetMPSComputeRunningProcesses_v3(nvmlDevice_t device,
        unsigned int *infoCount, void *infos) {
    (void)device; (void)infos;
    if (infoCount) *infoCount = 0;
    return NVML_SUCCESS;
}

nvmlReturn_t nvmlDeviceGetHandleByPciBusId_v2(const char *pciBusId, nvmlDevice_t *device) {
    (void)pciBusId; (void)device;
    return NVML_ERROR_NOT_SUPPORTED;
}

nvmlReturn_t nvmlDeviceGetGpuInstancePossiblePlacements_v2(nvmlDevice_t device,
        unsigned int profileId, void *placements, unsigned int *count) {
    (void)device; (void)profileId; (void)placements;
    if (count) *count = 0;
    return NVML_ERROR_NOT_SUPPORTED;
}

nvmlReturn_t nvmlVgpuInstanceGetLicenseInfo_v2(unsigned int vgpuInstance, void *info) {
    (void)vgpuInstance; (void)info;
    return NVML_ERROR_NOT_SUPPORTED;
}

nvmlReturn_t nvmlDeviceGetDriverModel_v2(nvmlDevice_t device, void *current, void *pending) {
    (void)device; (void)current; (void)pending;
    return NVML_ERROR_NOT_SUPPORTED;
}
