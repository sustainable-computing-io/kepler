package model

import "log"

const (
	RAPL_LABEL = "RAPL_POWER"
	PAPI_LABEL = "POWER"

	MODEL_DB_PATH = "../../data/modeldb"
)

// GetPower return power from provided features
func GetPower(valueMap map[string]float32) float32 {
	onlineTraining := false
	if _, exists := valueMap[RAPL_LABEL]; exists {
		onlineTraining = true
	} else if _, exists := valueMap[PAPI_LABEL]; exists {
		onlineTraining = true
	}
	if onlineTraining {
		AddTrainData(valueMap)
	}
	modelName, features, err := SelectModel(valueMap)
	// cannot select model
	if err != nil {
		log.Printf("%v", err)
		return -1
	}
	model, err := LoadPowerModel(modelName, features)
	// cannot load model
	if err != nil {
		log.Printf("%v", err)
		return -1
	}
	return model.GetPower(valueMap)
}
