package model

/*
#cgo CFLAGS: -I ./lib
*/

import (
	"fmt"
	"os"

	tflite "github.com/mattn/go-tflite"
)

const (
	MODEL_PATH = MODEL_DB_PATH + "/model"
)

type PowerModel struct {
	ModelName string
	Features  []string
	*tflite.Model
}

func LoadPowerModel(modelName string, features []string) (*PowerModel, error) {
	modelFilename := fmt.Sprintf("%s/%s", MODEL_PATH, modelName)
	_, err := os.Stat(modelFilename)
	if os.IsNotExist(err) {
		return &PowerModel{}, err
	}

	model := tflite.NewModelFromFile(modelFilename)
	return &PowerModel{
		ModelName: modelName,
		Features:  features,
		Model:     model,
	}, nil
}

func (m *PowerModel) GetPower(valueMap map[string]float32) float32 {
	options := tflite.NewInterpreterOptions()
	defer options.Delete()

	interpreter := tflite.NewInterpreter(m.Model, options)
	defer interpreter.Delete()

	// Create an input tensor
	interpreter.AllocateTensors()
	inputTensor := interpreter.GetInputTensor(0)
	float32s := inputTensor.Float32s()
	for index, feature := range m.Features {
		if value, exists := valueMap[feature]; exists {
			float32s[index] = value
		}
	}
	interpreter.Invoke()
	return interpreter.GetOutputTensor(0).Float32s()[0]
}

func (m *PowerModel) Close() {
	m.Model.Delete()
}
