package models

import "time"

// ActuatorLog mencatat setiap perubahan status aktuator.
// Source: "manual" (tombol dashboard), "auto" (aturan otomasi), "device" (laporan dari ESP32).
type ActuatorLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ActuatorID uint      `gorm:"index;not null" json:"actuator_id"`
	Actuator   Actuator  `json:"actuator"`
	State      bool      `json:"state"`
	Source     string    `gorm:"size:20;not null" json:"source"`
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
}
