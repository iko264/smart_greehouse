package api

import (
	"slices"

	"github.com/gofiber/fiber/v3"

	"smart-greenhouse/app/models"
	"smart-greenhouse/app/services/mqtt"
	"smart-greenhouse/config"
)

type SensorController struct{}

func NewSensorController() *SensorController {
	return &SensorController{}
}

// Latest: GET /api/sensors/latest
func (s *SensorController) Latest(c fiber.Ctx) error {
	var latest models.SensorReading
	if err := config.DB.Order("id desc").First(&latest).Error; err != nil {
		return c.JSON(fiber.Map{"data": nil})
	}
	return c.JSON(fiber.Map{"data": latest})
}

// History: GET /api/sensors/history?limit=30  (urut dari yang paling lama, siap untuk grafik)
func (s *SensorController) History(c fiber.Ctx) error {
	limit := fiber.Query[int](c, "limit", 30)
	if limit < 1 {
		limit = 1
	}
	if limit > 500 {
		limit = 500
	}

	var readings []models.SensorReading
	config.DB.Order("id desc").Limit(limit).Find(&readings)
	slices.Reverse(readings)

	return c.JSON(fiber.Map{"data": readings})
}

// Status: GET /api/status
func (s *SensorController) Status(c fiber.Ctx) error {
	var actuators []models.Actuator
	config.DB.Order("id").Find(&actuators)

	return c.JSON(fiber.Map{
		"mqtt_connected": mqtt.IsConnected(),
		"actuators":      actuators,
	})
}
