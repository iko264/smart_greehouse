package mqtt

import (
	"encoding/json"
	"errors"
	"log"
	"sync"
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

// PublishThresholds mengirim semua aturan otomasi ke ESP32 lewat topic
// <prefix>/config/thresholds (retained), jadi ESP32 baru menyala pun langsung
// menerima threshold terakhir. Dipanggil saat terhubung, saat aturan dibuat /
// diaktifkan / dinonaktifkan / dihapus, dan saat mode aktuator berubah.
func PublishThresholds() {
	if !IsConnected() {
		return
	}

	var rules []models.AutomationRule
	config.DB.Preload("Actuator").Where("is_active = ?", true).Find(&rules)

	byActuator := make(map[uint]models.AutomationRule, len(rules))
	for _, r := range rules {
		byActuator[r.ActuatorID] = r
	}

	var actuators []models.Actuator
	config.DB.Order("id").Find(&actuators)

	type th struct {
		Active   bool    `json:"active"`
		OnValue  float64 `json:"on_value"`
		OffValue float64 `json:"off_value"`
	}

	payload := map[string]th{}
	for _, a := range actuators {
		t := th{OnValue: 0, OffValue: 0}
		if r, ok := byActuator[a.ID]; ok && r.IsActive {
			t.Active = a.Mode == models.ModeAuto
			t.OnValue = r.OnValue
			t.OffValue = r.OffValue
		}
		payload[a.Code] = t
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Println("Gagal membuat JSON threshold:", err)
		return
	}

	client.Publish(Topic("config", "thresholds"), 1, true, body)
	log.Println("Threshold dikirim ke ESP32:", string(body))
}

// settingValue membaca satu pengaturan dari tabel greenhouse_settings.
func settingValue(key string, fallback float64) float64 {
	var s models.GreenhouseSetting
	if err := config.DB.Where("key = ?", key).First(&s).Error; err != nil {
		return fallback
	}
	return s.Value
}

// PublishLimits mengirim target jam cahaya harian & batas siram harian ke
// ESP32 lewat topic <prefix>/config/limits (retained). Dipanggil saat
// terhubung ke broker dan saat pengaturan disimpan dari dashboard.
func PublishLimits() {
	if !IsConnected() {
		return
	}

	lightTarget := settingValue(models.KeyLightTargetHours, 16)
	maxWater := settingValue(models.KeyMaxWatering, 6)

	body, err := json.Marshal(map[string]float64{
		"light_target_hours":   lightTarget,
		"max_watering_per_day": maxWater,
	})
	if err != nil {
		log.Println("Gagal membuat JSON limits:", err)
		return
	}

	client.Publish(Topic("config", "limits"), 1, true, body)
	log.Println("Limits dikirim ke ESP32:", string(body))
}

// GetLightTargetHours membaca target jam cahaya harian (default 16).
func GetLightTargetHours() float64 {
	return settingValue(models.KeyLightTargetHours, 16)
}

// GetMaxWatering membaca batas siram harian (default 6).
func GetMaxWatering() float64 {
	return settingValue(models.KeyMaxWatering, 6)
}

// dailyStatus menyimpan progres harian terakhir dari ESP32 (dibaca halaman & API).
var dailyStatus struct {
	mu           sync.Mutex
	lightSeconds uint32
	waterCount   int
	valid        bool
}

// SetDailyStatus menyimpan progres harian yang baru masuk dari topic status/daily.
func SetDailyStatus(lightSeconds uint32, waterCount int) {
	dailyStatus.mu.Lock()
	defer dailyStatus.mu.Unlock()

	if lightSeconds > 0 {
		dailyStatus.lightSeconds = lightSeconds
	}
	if waterCount >= 0 {
		dailyStatus.waterCount = waterCount
	}
	dailyStatus.valid = true
}

// GetDailyStatus mengembalikan progres harian terakhir (nilainya 0 jika belum ada data).
func GetDailyStatus() map[string]any {
	dailyStatus.mu.Lock()
	defer dailyStatus.mu.Unlock()

	if !dailyStatus.valid {
		return map[string]any{"light_seconds": 0, "water_count": 0}
	}
	return map[string]any{
		"light_seconds": dailyStatus.lightSeconds,
		"water_count":   dailyStatus.waterCount,
	}
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
