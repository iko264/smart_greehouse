package mqtt

import (
	"errors"
	"time"

	"smart-greenhouse/app/models"
	"smart-greenhouse/app/services/websocket"
	"smart-greenhouse/config"
)

// SetActuator mengirim perintah ON/OFF ke ESP32 lewat topic <prefix>/actuators/<kode>/set,
// lalu menyimpan status baru + log + memberi tahu browser.
//
// source: "manual" (tombol dashboard) atau "auto" (automation engine).
func SetActuator(code string, on bool, source string) error {
	if !IsConnected() {
		return errors.New("MQTT belum terhubung ke broker")
	}

	var actuator models.Actuator
	if err := config.DB.Where("code = ?", code).First(&actuator).Error; err != nil {
		return errors.New("aktuator tidak ditemukan")
	}

	payload := "OFF"
	if on {
		payload = "ON"
	}

	token := client.Publish(Topic("actuators", code, "set"), 1, false, payload)
	if !token.WaitTimeout(3 * time.Second) {
		return errors.New("timeout saat mengirim perintah ke broker")
	}
	if err := token.Error(); err != nil {
		return err
	}

	return saveState(&actuator, on, source)
}

// saveState menyimpan status aktuator ke database dan mengabari browser.
// Kalau status tidak berubah, tidak ada yang dilakukan (mencegah log ganda saat ESP32 membalas perintah).
func saveState(actuator *models.Actuator, on bool, source string) error {
	if actuator.State == on {
		return nil
	}

	if err := config.DB.Model(actuator).Update("state", on).Error; err != nil {
		return err
	}
	actuator.State = on

	config.DB.Create(&models.ActuatorLog{ActuatorID: actuator.ID, State: on, Source: source})

	websocket.H.Broadcast("actuator", actuator)
	return nil
}
