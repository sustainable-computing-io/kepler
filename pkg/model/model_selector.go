package model

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

const (
	METADATA_PATH = MODEL_DB_PATH + "/metadata"
	ERROR_METRIC  = "mae"
	FEATURE_META  = "features"
)

func isApplicable(metadata map[string]interface{}, valueMap map[string]float32) (bool, []string) {
	if features, exists := metadata[FEATURE_META]; exists {
		featureInterfaceList := features.([]interface{})
		featureList := []string{}
		for _, feature := range featureInterfaceList {
			if _, exists := valueMap[feature.(string)]; !exists {
				return false, []string{}
			}
			featureList = append(featureList, feature.(string))
		}
		return true, featureList
	} else {
		// no feature in metadata
		return false, []string{}
	}
}

func SelectModel(valueMap map[string]float32) (string, []string, error) {
	selectedModelName := ""
	var selectedFeatures []string
	files, err := ioutil.ReadDir(METADATA_PATH)
	// fail to list model metadata
	if err != nil {
		return "", []string{}, err
	}

	minErrValue := -1.0
	for _, file := range files {
		fileName := file.Name()
		if strings.Contains(fileName, ".json") {
			var metadata map[string]interface{}
			jsonPath := METADATA_PATH + "/" + fileName
			jsonFile, err := os.Open(jsonPath)
			if err != nil {
				continue
			}
			byteValue, _ := ioutil.ReadAll(jsonFile)
			json.Unmarshal(byteValue, &metadata)
			if errValue, exists := metadata[ERROR_METRIC]; exists {
				errValueInFloat := errValue.(float64)
				modelName := strings.ReplaceAll(fileName, ".json", ".tflite")
				if valid, features := isApplicable(metadata, valueMap); valid {
					if selectedModelName == "" || errValueInFloat < minErrValue {
						selectedModelName = modelName
						minErrValue = errValueInFloat
						selectedFeatures = features
					}
				}
			}
		}
	}
	if selectedModelName != "" {
		return selectedModelName, selectedFeatures, nil
	}
	return selectedModelName, []string{}, fmt.Errorf("No model is selected")
}
