package controllers

import (
	"github.com/gofiber/fiber/v3"

	"smart-greenhouse/app/http/requests"
	"smart-greenhouse/app/models"
	"smart-greenhouse/app/support"
	"smart-greenhouse/config"
)

type AutomationController struct{}

func NewAutomationController() *AutomationController {
	return &AutomationController{}
}

func (a *AutomationController) Index(c fiber.Ctx) error {
	return a.render(c, requests.AutomationRuleRequest{}, map[string]string{})
}

// render dipakai bersama oleh Index dan Store (saat validasi gagal).
func (a *AutomationController) render(c fiber.Ctx, old requests.AutomationRuleRequest, errs map[string]string) error {
	var rules []models.AutomationRule
	config.DB.Preload("Actuator").Order("id").Find(&rules)

	var actuators []models.Actuator
	config.DB.Order("id").Find(&actuators)

	return support.View(c, "automation/index", fiber.Map{
		"title":     "Otomasi",
		"rules":     rules,
		"actuators": actuators,
		"sensors":   support.Sensors,
		"old":       old,
		"errors":    errs,
	})
}

func (a *AutomationController) Store(c fiber.Ctx) error {
	var req requests.AutomationRuleRequest
	if err := c.Bind().Form(&req); err != nil {
		return fiber.ErrBadRequest
	}

	errs := support.Validate(req)

	// Aturan histeresis: batas nyala dan batas mati harus berbeda arah.
	if len(errs) == 0 {
		if req.Direction == "above" && req.OnValue <= req.OffValue {
			errs["on_value"] = "Untuk kondisi 'di atas', batas nyala harus lebih besar dari batas mati"
		}
		if req.Direction == "below" && req.OnValue >= req.OffValue {
			errs["on_value"] = "Untuk kondisi 'di bawah', batas nyala harus lebih kecil dari batas mati"
		}
	}

	var actuator models.Actuator
	if len(errs) == 0 {
		if err := config.DB.First(&actuator, req.ActuatorID).Error; err != nil {
			errs["actuator_id"] = "Aktuator tidak ditemukan"
		}
	}

	if len(errs) > 0 {
		c.Status(fiber.StatusUnprocessableEntity)
		return a.render(c, req, errs)
	}

	rule := models.AutomationRule{
		Name:       req.Name,
		ActuatorID: actuator.ID,
		Sensor:     req.Sensor,
		Direction:  req.Direction,
		OnValue:    req.OnValue,
		OffValue:   req.OffValue,
		IsActive:   true,
	}
	if err := config.DB.Create(&rule).Error; err != nil {
		return err
	}

	support.Flash(c, "success", "Aturan otomasi ditambahkan.")
	return c.Redirect().To("/automation")
}

// Toggle: aktifkan / nonaktifkan aturan.
func (a *AutomationController) Toggle(c fiber.Ctx) error {
	id := fiber.Params[int](c, "id", 0)
	if id == 0 {
		return fiber.ErrNotFound
	}

	var rule models.AutomationRule
	if err := config.DB.First(&rule, id).Error; err != nil {
		return fiber.ErrNotFound
	}

	config.DB.Model(&rule).Update("is_active", !rule.IsActive)

	support.Flash(c, "success", "Status aturan diperbarui.")
	return c.Redirect().To("/automation")
}

func (a *AutomationController) Destroy(c fiber.Ctx) error {
	id := fiber.Params[int](c, "id", 0)
	if id == 0 {
		return fiber.ErrNotFound
	}

	config.DB.Delete(&models.AutomationRule{}, id)

	support.Flash(c, "success", "Aturan dihapus.")
	return c.Redirect().To("/automation")
}
