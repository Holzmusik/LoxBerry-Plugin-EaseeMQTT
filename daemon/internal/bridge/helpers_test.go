package bridge

import (
	"time"

	"easeemqtt/internal/config"
	"easeemqtt/internal/easee"
)

func testConfig() *config.Config {
	return &config.Config{
		Easee: config.EaseeConfig{Chargers: []config.Charger{{ID: "EH1"}, {ID: "EH2"}}},
		MQTT:  config.MQTTConfig{TopicPrefix: "easee/"},
	}
}

func obs(charger string, id int, value string) easee.Observation {
	return easee.Observation{ChargerID: charger, ID: id, Value: value, Timestamp: time.Now()}
}
