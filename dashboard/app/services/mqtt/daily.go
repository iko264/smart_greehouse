package mqtt

import (
	"log"
	"time"

	"gorm.io/gorm/clause"

	"smart-greenhouse/app/models"
	"smart-greenhouse/config"
)

// SaveDailySummary menyimpan ulang rekap hari ini (zona WIB) berdasarkan
// progres dari ESP8266 dan target/batas yang sedang berlaku.
func SaveDailySummary(lightSeconds uint32, waterCount int) {
	summary := models.DailySummary{
		Date:             time.Now().In(time.Local).Format("2006-01-02"),
		LightSeconds:     lightSeconds,
		LightTargetHours: GetLightTargetHours(),
		WaterCount:       waterCount,
		MaxWatering:      int(GetMaxWatering()),
	}

	err := config.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "date"}},
		DoUpdates: clause.Assignments(map[string]any{
			"light_seconds":        summary.LightSeconds,
			"light_target_hours":   summary.LightTargetHours,
			"water_count":          summary.WaterCount,
			"max_watering_per_day": summary.MaxWatering,
			"updated_at":           time.Now(),
		}),
	}).Create(&summary).Error
	if err != nil {
		log.Println("Gagal menyimpan rekap harian:", err)
	}
}

// RefreshDailySummary memperbarui rekap hari ini dengan limit terbaru.
// Dipanggil setelah pengaturan disimpan dari dashboard.
func RefreshDailySummary() {
	dailyStatus.mu.Lock()
	lightSeconds := dailyStatus.lightSeconds
	waterCount := dailyStatus.waterCount
	valid := dailyStatus.valid
	dailyStatus.mu.Unlock()

	if !valid {
		return
	}
	SaveDailySummary(lightSeconds, waterCount)
}
