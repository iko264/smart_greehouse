package controllers

import (
	"github.com/gofiber/fiber/v3"

	"smart-greenhouse/app/http/requests"
	"smart-greenhouse/app/models"
	"smart-greenhouse/app/services/mqtt"
	"smart-greenhouse/app/support"
	"smart-greenhouse/config"
)

type GreenhouseController struct{}

func NewGreenhouseController() *GreenhouseController {
	return &GreenhouseController{}
}

// LimitsPage menampilkan halaman "Pengaturan Harian" (card cahaya & penyiraman).
func (g *GreenhouseController) LimitsPage(c fiber.Ctx) error {
	return support.View(c, "greenhouse/limits", fiber.Map{
		"title":            "Manajemen Cahaya & Penyiraman",
		"lightTargetHours": mqtt.GetLightTargetHours(),
		"maxWatering":      int(mqtt.GetMaxWatering()),
	})
}

// Limits: POST /greenhouse/limits - simpan pengaturan lalu kirim ke ESP8266.
func (g *GreenhouseController) Limits(c fiber.Ctx) error {
	var req requests.GreenhouseLimitsRequest
	if err := c.Bind().Form(&req); err != nil {
		return support.JSONError(c, fiber.StatusBadRequest, "Request tidak valid")
	}
	if errs := support.Validate(req); len(errs) > 0 {
		if msg, ok := errs["LightTargetHours"]; ok {
			return support.JSONError(c, fiber.StatusUnprocessableEntity, msg)
		}
		if msg, ok := errs["MaxWatering"]; ok {
			return support.JSONError(c, fiber.StatusUnprocessableEntity, msg)
		}
		return support.JSONError(c, fiber.StatusUnprocessableEntity, "Data tidak valid")
	}

	updates := []models.GreenhouseSetting{
		{Key: models.KeyLightTargetHours, Value: req.LightTargetHours},
		{Key: models.KeyMaxWatering, Value: float64(req.MaxWatering)},
	}

	for _, u := range updates {
		var s models.GreenhouseSetting
		if err := config.DB.Where(map[string]any{"key": u.Key}).First(&s).Error; err != nil {
			if err := config.DB.Create(&models.GreenhouseSetting{Key: u.Key, Value: u.Value}).Error; err != nil {
				return support.JSONError(c, fiber.StatusInternalServerError, "Gagal menyimpan pengaturan")
			}
			continue
		}
		if err := config.DB.Model(&s).Updates(map[string]any{"value": u.Value}).Error; err != nil {
			return support.JSONError(c, fiber.StatusInternalServerError, "Gagal menyimpan pengaturan")
		}
	}

	mqtt.RefreshDailySummary() // perbarui rekap hari ini dengan limit baru
	mqtt.PublishLimits()       // kirim nilai baru ke ESP8266 (retained)

	return c.JSON(fiber.Map{
		"message": "Pengaturan tersimpan dan dikirim ke perangkat",
		"data": fiber.Map{
			"light_target_hours":   req.LightTargetHours,
			"max_watering_per_day": req.MaxWatering,
		},
	})
}

// LimitsAPI: GET /api/greenhouse/limits - nilai pengaturan saat ini (JSON).
func (g *GreenhouseController) LimitsAPI(c fiber.Ctx) error {
	return c.JSON(fiber.Map{"data": fiber.Map{
		"light_target_hours":   mqtt.GetLightTargetHours(),
		"max_watering_per_day": int(mqtt.GetMaxWatering()),
	}})
}

// DailyAPI: GET /api/greenhouse/daily - progres harian terakhir dari ESP8266.
func (g *GreenhouseController) DailyAPI(c fiber.Ctx) error {
	return c.JSON(fiber.Map{"data": mqtt.GetDailyStatus()})
}
