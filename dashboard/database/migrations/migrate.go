package migrations

import (
	"log"

	"gorm.io/gorm"

	"smart-greenhouse/app/models"
)

// Run membuat/menyesuaikan tabel dari struct model (AutoMigrate).
func Run(db *gorm.DB) {
	err := db.AutoMigrate(
		&models.User{},
		&models.SensorReading{},
		&models.Actuator{},
		&models.ActuatorLog{},
		&models.AutomationRule{},
		&models.GreenhouseSetting{},
	)
	if err != nil {
		log.Fatalf("Migrasi gagal: %v", err)
	}
	log.Println("Migrasi selesai")
}
