package models

import "time"

const (
	KeyLightTargetHours = "light_target_hours"
	KeyMaxWatering      = "max_watering_per_day"
)

// GreenhouseSetting menyimpan konfigurasi greenhouse berupa pasangan
// key-value (target jam cahaya harian, batas siram harian, dst).
type GreenhouseSetting struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Key       string    `gorm:"size:50;uniqueIndex;not null" json:"key"`
	Value     float64   `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}
