package models

import "time"

const (
	ModeManual = "manual" // dikontrol lewat tombol di dashboard
	ModeAuto   = "auto"   // dikontrol oleh aturan otomasi
)

// Actuator adalah perangkat yang bisa dinyalakan/dimatikan (kipas, pompa, lampu).
type Actuator struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Code      string    `gorm:"size:50;uniqueIndex;not null" json:"code"` // dipakai di topic MQTT, mis. "fan"
	Name      string    `gorm:"size:100;not null" json:"name"`
	Icon      string    `gorm:"size:50" json:"icon"`
	State     bool      `json:"state"` // true = ON
	Mode      string    `gorm:"size:10;not null" json:"mode"`
	UpdatedAt time.Time `json:"updated_at"`
}
