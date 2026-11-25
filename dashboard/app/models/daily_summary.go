package models

import (
	"strconv"
	"time"
)

// DailySummary adalah rekap harian manajemen cahaya dan penyiraman.
// Satu baris mewakili satu tanggal (zona WIB), berisi total yang dilaporkan
// ESP8266 beserta target/batas yang berlaku saat itu.
type DailySummary struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	Date             string    `gorm:"size:10;uniqueIndex;not null" json:"date"` // YYYY-MM-DD
	LightSeconds     uint32    `json:"light_seconds"`
	LightTargetHours float64   `json:"light_target_hours"`
	WaterCount       int       `json:"water_count"`
	MaxWatering      int       `gorm:"column:max_watering_per_day" json:"max_watering_per_day"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// DateLabel menampilkan tanggal dalam format yang ramah dibaca.
func (d DailySummary) DateLabel() string {
	t, err := time.Parse("2006-01-02", d.Date)
	if err != nil {
		return d.Date
	}
	return t.Format("02 Jan 2006")
}

// LightHoursText mengubah total detik cahaya menjadi jam dengan satu desimal.
func (d DailySummary) LightHoursText() string {
	return strconv.FormatFloat(float64(d.LightSeconds)/3600, 'f', 1, 64)
}

// LightTargetText menampilkan target jam cahaya tanpa desimal yang tidak perlu.
func (d DailySummary) LightTargetText() string {
	return strconv.FormatFloat(d.LightTargetHours, 'f', -1, 64)
}

// LightAchieved bernilai true jika total cahaya sudah mencapai target.
func (d DailySummary) LightAchieved() bool {
	return float64(d.LightSeconds) >= d.LightTargetHours*3600
}

// WaterLimitReached bernilai true jika jumlah siram sudah mencapai batas harian.
func (d DailySummary) WaterLimitReached() bool {
	return d.WaterCount >= d.MaxWatering
}
