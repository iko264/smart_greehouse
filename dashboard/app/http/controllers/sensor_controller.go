package controllers

import (
	"math"

	"github.com/gofiber/fiber/v3"

	"smart-greenhouse/app/models"
	"smart-greenhouse/app/support"
	"smart-greenhouse/config"
)

type SensorController struct{}

func NewSensorController() *SensorController {
	return &SensorController{}
}

// Monitoring: halaman grafik per sensor (data grafik diambil JavaScript dari /api/sensors/history).
func (s *SensorController) Monitoring(c fiber.Ctx) error {
	return support.View(c, "monitoring/index", fiber.Map{
		"title":   "Monitoring",
		"sensors": support.Sensors,
	})
}

// History: riwayat data sensor dan log aktuator, dengan pagination.
func (s *SensorController) History(c fiber.Ctx) error {
	const perPage = 20

	tab := c.Query("tab", "sensor")
	page := fiber.Query[int](c, "page", 1)
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * perPage

	data := fiber.Map{"title": "Riwayat"}
	var total int64

	if tab == "actuator" {
		var logs []models.ActuatorLog
		config.DB.Model(&models.ActuatorLog{}).Count(&total)
		config.DB.Preload("Actuator").Order("id desc").Limit(perPage).Offset(offset).Find(&logs)
		data["logs"] = logs
	} else {
		tab = "sensor"
		var readings []models.SensorReading
		config.DB.Model(&models.SensorReading{}).Count(&total)
		config.DB.Order("id desc").Limit(perPage).Offset(offset).Find(&readings)
		data["readings"] = readings
	}

	totalPages := int(math.Ceil(float64(total) / float64(perPage)))
	if totalPages < 1 {
		totalPages = 1
	}

	data["tab"] = tab
	data["page"] = page
	data["total"] = total
	data["totalPages"] = totalPages
	data["hasPrev"] = page > 1
	data["hasNext"] = page < totalPages
	data["prev"] = page - 1
	data["next"] = page + 1

	return support.View(c, "history/index", data)
}
