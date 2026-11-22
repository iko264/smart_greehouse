package controllers

import (
	"github.com/gofiber/fiber/v3"

	"smart-greenhouse/app/http/requests"
	"smart-greenhouse/app/models"
	"smart-greenhouse/app/services/mqtt"
	ws "smart-greenhouse/app/services/websocket"
	"smart-greenhouse/app/support"
	"smart-greenhouse/config"
)

type ActuatorController struct{}

func NewActuatorController() *ActuatorController {
	return &ActuatorController{}
}

func (a *ActuatorController) Index(c fiber.Ctx) error {
	var actuators []models.Actuator
	config.DB.Order("id").Find(&actuators)

	var logs []models.ActuatorLog
	config.DB.Preload("Actuator").Order("id desc").Limit(10).Find(&logs)

	return support.View(c, "actuators/index", fiber.Map{
		"title":     "Kontrol Aktuator",
		"actuators": actuators,
		"logs":      logs,
	})
}

// Command: POST /actuators/:code/command  (dipanggil JavaScript, balasan JSON)
func (a *ActuatorController) Command(c fiber.Ctx) error {
	var req requests.ActuatorCommandRequest
	if err := c.Bind().Form(&req); err != nil {
		return support.JSONError(c, fiber.StatusBadRequest, "Request tidak valid")
	}
	if errs := support.Validate(req); len(errs) > 0 {
		return support.JSONError(c, fiber.StatusUnprocessableEntity, errs["state"])
	}

	var actuator models.Actuator
	if err := config.DB.Where("code = ?", c.Params("code")).First(&actuator).Error; err != nil {
		return support.JSONError(c, fiber.StatusNotFound, "Aktuator tidak ditemukan")
	}
	if actuator.Mode == models.ModeAuto {
		return support.JSONError(c, fiber.StatusConflict, actuator.Name+" sedang mode otomatis. Ubah ke manual dulu.")
	}

	if err := mqtt.SetActuator(actuator.Code, req.State == "on", "manual"); err != nil {
		return support.JSONError(c, fiber.StatusBadGateway, err.Error())
	}

	config.DB.First(&actuator, actuator.ID) // ambil status terbaru
	return c.JSON(fiber.Map{"message": "Perintah terkirim", "data": actuator})
}

// Mode: POST /actuators/:code/mode  (manual / auto)
func (a *ActuatorController) Mode(c fiber.Ctx) error {
	var req requests.ActuatorModeRequest
	if err := c.Bind().Form(&req); err != nil {
		return support.JSONError(c, fiber.StatusBadRequest, "Request tidak valid")
	}
	if errs := support.Validate(req); len(errs) > 0 {
		return support.JSONError(c, fiber.StatusUnprocessableEntity, errs["mode"])
	}

	var actuator models.Actuator
	if err := config.DB.Where("code = ?", c.Params("code")).First(&actuator).Error; err != nil {
		return support.JSONError(c, fiber.StatusNotFound, "Aktuator tidak ditemukan")
	}

	if err := config.DB.Model(&actuator).Update("mode", req.Mode).Error; err != nil {
		return support.JSONError(c, fiber.StatusInternalServerError, "Gagal mengubah mode")
	}
	actuator.Mode = req.Mode

	mqtt.PublishThresholds() // aktif/inaktifnya otomasi di ESP32 ikut mode

	ws.H.Broadcast("actuator", actuator) // browser lain ikut ter-update
	return c.JSON(fiber.Map{"message": "Mode diubah", "data": actuator})
}
