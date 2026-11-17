package models

import "time"

// SensorReading adalah satu paket data sensor dari ESP32.
type SensorReading struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Temperature float64   `json:"temperature"` // suhu udara (°C)
	Humidity    float64   `json:"humidity"`    // kelembapan udara (%)
	Soil        float64   `json:"soil"`        // kelembapan tanah (%)
	Light       float64   `json:"light"`       // intensitas cahaya (LDR)
	CreatedAt   time.Time `gorm:"index" json:"created_at"`
}

// Value mengambil nilai sensor berdasarkan kodenya ("temperature", "humidity", ...).
// Dipakai oleh automation engine dan template.
func (r SensorReading) Value(sensor string) float64 {
	switch sensor {
	case "temperature":
		return r.Temperature
	case "humidity":
		return r.Humidity
	case "soil":
		return r.Soil
	case "light":
		return r.Light
	}
	return 0
}
