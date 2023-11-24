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

package local

/*
#cgo CFLAGS: -I/usr/local/include
#cgo LDFLAGS: -lxgboost -L/usr/local/lib64
#include <stdio.h>
#include <stdlib.h>
#include <xgboost/c_api.h>

// load model from json file
BoosterHandle LoadModelFromJSON(const char* filename) {
    BoosterHandle h_booster;
    XGBoosterCreate(NULL, 0, &h_booster);
    XGBoosterLoadModel(h_booster, filename);
    return h_booster;
}

// load model from buffer
BoosterHandle LoadModelFromBuffer(const char* buffer, int len) {
	BoosterHandle h_booster;
	XGBoosterCreate(NULL, 0, &h_booster);
	XGBoosterLoadModelFromBuffer(h_booster, buffer, len);
	return h_booster;
}

int GetNumFeatures(BoosterHandle h_booster) {
    bst_ulong num_features;
    XGBoosterGetNumFeature(h_booster, &num_features);
    return num_features;
}

int Predict(BoosterHandle h_booster, float *data, int rows, int cols, float *output) {
    DMatrixHandle h_mat;
    XGDMatrixCreateFromMat((float *)data, rows, cols, -1, &h_mat);
    bst_ulong out_len;
    const float *f;
    XGBoosterPredict(h_booster, h_mat, 0, 0, 0, &out_len, &f);
    if (out_len != rows) {
        printf("error: output length is not equal to input rows\n");
        XGDMatrixFree(h_mat);
        return 0;
    }
    for (int i = 0; i < out_len; i++) {
        output[i] = f[i];
    }
    XGDMatrixFree(h_mat);
    return out_len;
}
*/
import "C"

import (
	"encoding/base64"
	"fmt"
	"reflect"
	"unsafe"
)

type XGBoostModelWeight struct {
	hBooster     C.BoosterHandle
	num_features int
}

func (m *XGBoostModelWeight) LoadFromJson(modelPath string) error {
	m.hBooster = C.LoadModelFromJSON(C.CString(modelPath))
	if reflect.ValueOf(m.hBooster).IsNil() {
		return fmt.Errorf("load model from json %s failed", modelPath)
	}
	m.num_features = int(C.GetNumFeatures(m.hBooster))
	return nil
}

func (m *XGBoostModelWeight) LoadFromBuffer(modelString string) error {
	// model is a base64 encoded string, decode it first
	decodedLearner, err := base64.StdEncoding.DecodeString(modelString)
	if err != nil {
		return fmt.Errorf("decode learner failed: %v", err)
	}
	modelBuffer := []byte(decodedLearner)

	m.hBooster = C.LoadModelFromBuffer((*C.char)(unsafe.Pointer(&modelBuffer[0])), C.int(len(modelBuffer)))
	if reflect.ValueOf(m.hBooster).IsNil() {
		return fmt.Errorf("load model from buffer failed")
	}
	m.num_features = int(C.GetNumFeatures(m.hBooster))
	return nil
}

func (m *XGBoostModelWeight) Close() {
	C.XGBoosterFree(m.hBooster)
}

func (m *XGBoostModelWeight) PredictFromData(data []float32) ([]float64, error) {
	if len(data)%m.num_features != 0 {
		return nil, fmt.Errorf("data length (%d) is not mutiple of num of features (%d)", len(data), m.num_features)
	}
	rows := len(data) / m.num_features

	output := C.malloc(C.size_t(rows) * C.size_t(C.sizeof_float))
	out_len := int(C.Predict(m.hBooster, (*C.float)(&data[0]), /* input */
		C.int(rows) /* rows */, C.int(m.num_features) /* cols */, (*C.float)(output) /* predict outpout */))
	if out_len < 1 {
		return nil, fmt.Errorf("predict failed")
	}
	output_array := make([]float64, out_len)
	for i := 0; i < out_len; i++ {
		output_array[i] = float64(*(*float32)(unsafe.Pointer(uintptr(output) + uintptr(i)*unsafe.Sizeof(float32(0)))))
	}
	C.free(unsafe.Pointer(output))
	return output_array, nil
}
