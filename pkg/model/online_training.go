package model

const (
	TRAIN_BATCH_NUM = 100
)

var collector *DataCollector

type DataCollector struct {
	TrainData []map[string]float32
}

func InitDataCollector() {
	collector = &DataCollector{}
}

func AddTrainData(valueMap map[string]float32) {
	collector.Add(valueMap)
}

func (c *DataCollector) Add(valueMap map[string]float32) {
	collector.TrainData = append(collector.TrainData, valueMap)
	if len(collector.TrainData) >= TRAIN_BATCH_NUM {
		// TO-DO:
		// - send train data to model server
		// - update model and metadata in model db

		// clear data
		c.TrainData = []map[string]float32{}
	}
}
