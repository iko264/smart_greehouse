package models

import "time"

// AutomationRule adalah aturan otomasi dengan histeresis (dua batas) supaya aktuator tidak "berkedip".
//
//	Direction "above": nyala jika nilai >= OnValue, mati jika nilai <= OffValue  (contoh: kipas, suhu 30 / 28)
//	Direction "below": nyala jika nilai <= OnValue, mati jika nilai >= OffValue  (contoh: pompa, tanah 40 / 60)
type AutomationRule struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Name       string    `gorm:"size:100;not null" json:"name"`
	ActuatorID uint      `gorm:"index;not null" json:"actuator_id"`
	Actuator   Actuator  `json:"actuator"`
	Sensor     string    `gorm:"size:20;not null" json:"sensor"`
	Direction  string    `gorm:"size:10;not null" json:"direction"`
	OnValue    float64   `json:"on_value"`
	OffValue   float64   `json:"off_value"`
	IsActive   bool      `json:"is_active"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
