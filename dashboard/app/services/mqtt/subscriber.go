package mqtt

import (
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"

	"smart-greenhouse/app/models"
	"smart-greenhouse/app/services/websocket"
	"smart-greenhouse/config"
)

// OnSensor diisi saat bootstrap dengan automation engine.
// Dibuat sebagai variabel fungsi supaya package mqtt tidak perlu meng-import automation
// (kalau tidak, Go akan menolak karena import saling berputar).
var OnSensor func(reading models.SensorReading)

var (
	saveMu   sync.Mutex
	lastSave time.Time
)

func subscribeAll(c paho.Client) {
	handlers := map[string]paho.MessageHandler{
		Topic("sensors"):                 handleSensor,        // JSON dari ESP32
		Topic("actuators", "+", "state"): handleActuatorState, // "ON"/"OFF" dari ESP32
	}

	for topic, handler := range handlers {
		token := c.Subscribe(topic, 1, handler)
		token.Wait()
		if err := token.Error(); err != nil {
			log.Printf("Gagal subscribe %s: %v", topic, err)
			continue
		}
		log.Println("Subscribe:", topic)
	}
}

// handleSensor: <prefix>/sensors
// Contoh payload: {"temperature":28.5,"humidity":70.2,"soil":45,"light":600}
func handleSensor(_ paho.Client, msg paho.Message) {
	var p struct {
		Temperature *float64 `json:"temperature"`
		Humidity    *float64 `json:"humidity"`
		Soil        *float64 `json:"soil"`
		Light       *float64 `json:"light"`
	}
	if err := json.Unmarshal(msg.Payload(), &p); err != nil {
		log.Printf("Payload sensor bukan JSON yang valid: %s", msg.Payload())
		return
	}
	if p.Temperature == nil || p.Humidity == nil || p.Soil == nil || p.Light == nil {
		log.Printf("Payload sensor belum lengkap: %s", msg.Payload())
		return
	}

	reading := models.SensorReading{
		Temperature: *p.Temperature,
		Humidity:    *p.Humidity,
		Soil:        *p.Soil,
		Light:       *p.Light,
		CreatedAt:   time.Now(),
	}

	// 1. Simpan ke database (dibatasi interval supaya tabel tidak membengkak)
	if shouldSave() {
		if err := config.DB.Create(&reading).Error; err != nil {
			log.Println("Gagal menyimpan data sensor:", err)
		}
	}

	// 2. Olah data (moving average + z-score), lalu kirim ke semua browser lewat WebSocket
	event := analyze(reading)
	websocket.H.Broadcast("sensor", event)

	for code, isAnomaly := range event.Anomaly {
		if isAnomaly {
			log.Printf("Anomali %s: nilai=%.1f, z=%.2f", code, reading.Value(code), event.Z[code])
		}
	}

	// 3. Jalankan otomasi. Pakai goroutine karena callback MQTT sebaiknya tidak menunggu proses lain.
	if OnSensor != nil {
		go OnSensor(reading)
	}
}

// handleActuatorState: <prefix>/actuators/<kode>/state
// Dikirim ESP32 setiap status aktuator berubah, jadi dashboard selalu sama dengan kondisi alat sebenarnya.
func handleActuatorState(_ paho.Client, msg paho.Message) {
	parts := strings.Split(msg.Topic(), "/")
	if len(parts) < 2 {
		return
	}
	code := parts[len(parts)-2]

	on, ok := parseState(string(msg.Payload()))
	if !ok {
		log.Printf("Status aktuator tidak dikenal (%s): %s", code, msg.Payload())
		return
	}

	var actuator models.Actuator
	if err := config.DB.Where("code = ?", code).First(&actuator).Error; err != nil {
		log.Println("Aktuator tidak terdaftar di database:", code)
		return
	}

	if err := saveState(&actuator, on, "device"); err != nil {
		log.Println("Gagal menyimpan status aktuator:", err)
	}
}

func parseState(payload string) (on bool, ok bool) {
	switch strings.ToUpper(strings.TrimSpace(payload)) {
	case "ON", "1", "TRUE":
		return true, true
	case "OFF", "0", "FALSE":
		return false, true
	}
	return false, false
}

func shouldSave() bool {
	interval := time.Duration(config.EnvInt("SENSOR_SAVE_INTERVAL", 10)) * time.Second

	saveMu.Lock()
	defer saveMu.Unlock()

	if time.Since(lastSave) < interval {
		return false
	}
	lastSave = time.Now()
	return true
}
