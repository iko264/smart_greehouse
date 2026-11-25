package seeders

import (
	"log"

	"gorm.io/gorm"

	"smart-greenhouse/app/models"
	"smart-greenhouse/app/support"
	"smart-greenhouse/config"
)

// Run mengisi data awal. Aman dijalankan berkali-kali (tidak membuat data ganda).
func Run(db *gorm.DB) {
	seedUser(db)
	seedActuators(db)
	seedRules(db)
	seedSettings(db)
}

func seedUser(db *gorm.DB) {
	email := config.Env("ADMIN_EMAIL", "admin@greenhouse.test")

	var count int64
	db.Model(&models.User{}).Where("email = ?", email).Count(&count)
	if count > 0 {
		return
	}

	hash, err := support.HashPassword(config.Env("ADMIN_PASSWORD", "password"))
	if err != nil {
		log.Fatalf("Gagal membuat hash password: %v", err)
	}

	db.Create(&models.User{Name: "Administrator", Email: email, Password: hash})
	log.Println("User awal dibuat:", email)
}

func seedActuators(db *gorm.DB) {
	actuators := []models.Actuator{
		{Code: "fan", Name: "Kipas", Icon: "bi-fan", Mode: models.ModeManual},
		{Code: "pump", Name: "Pompa Air", Icon: "bi-droplet-half", Mode: models.ModeManual},
		{Code: "light", Name: "Lampu Tumbuh", Icon: "bi-lightbulb", Mode: models.ModeManual},
	}

	for _, a := range actuators {
		db.Where("code = ?", a.Code).FirstOrCreate(&a)
	}
}

func seedRules(db *gorm.DB) {
	var count int64
	db.Model(&models.AutomationRule{}).Count(&count)
	if count > 0 {
		return
	}

	// Aturan contoh. Baru bekerja kalau aktuatornya diubah ke mode "Otomatis".
	examples := []struct {
		code string
		rule models.AutomationRule
	}{
		{"fan", models.AutomationRule{Name: "Kipas saat panas", Sensor: "temperature", Direction: "above", OnValue: 30, OffValue: 28}},
		{"pump", models.AutomationRule{Name: "Pompa saat tanah kering", Sensor: "soil", Direction: "below", OnValue: 40, OffValue: 60}},
		{"light", models.AutomationRule{Name: "Lampu saat gelap", Sensor: "light", Direction: "below", OnValue: 300, OffValue: 500}},
	}

	for _, e := range examples {
		var actuator models.Actuator
		if err := db.Where("code = ?", e.code).First(&actuator).Error; err != nil {
			continue
		}
		e.rule.ActuatorID = actuator.ID
		e.rule.IsActive = true
		db.Create(&e.rule)
	}
}

func seedSettings(db *gorm.DB) {
	defaults := []models.GreenhouseSetting{
		{Key: models.KeyLightTargetHours, Value: 16},
		{Key: models.KeyMaxWatering, Value: 6},
	}

	for _, s := range defaults {
		db.Where(map[string]any{"key": s.Key}).FirstOrCreate(&s)
	}
}
