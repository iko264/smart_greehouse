package controllers

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"

	"smart-greenhouse/app/models"
	"smart-greenhouse/app/support"
	"smart-greenhouse/config"
)

type DashboardController struct{}

func NewDashboardController() *DashboardController {
	return &DashboardController{}
}

// statRow adalah satu baris tabel ringkasan 24 jam.
type statRow struct {
	Label string
	Unit  string
	Min   string
	Avg   string
	Max   string
}

func (d *DashboardController) Index(c fiber.Ctx) error {
	var latest models.SensorReading
	hasData := config.DB.Order("id desc").First(&latest).Error == nil

	var actuators []models.Actuator
	config.DB.Order("id").Find(&actuators)

	return support.View(c, "dashboard/index", fiber.Map{
		"title":     "Dashboard",
		"latest":    latest,
		"hasData":   hasData,
		"actuators": actuators,
		"sensors":   support.Sensors,
		"stats":     buildStats(),
	})
}

// buildStats menghitung nilai terendah, rata-rata, dan tertinggi tiap sensor dalam 24 jam terakhir
// langsung di MySQL (MIN, AVG, MAX).
func buildStats() []statRow {
	since := time.Now().Add(-24 * time.Hour)
	rows := make([]statRow, 0, len(support.Sensors))

	for _, s := range support.Sensors {
		var result struct {
			Min *float64
			Avg *float64
			Max *float64
		}

		// s.Code berasal dari daftar tetap di kode (bukan input user), jadi aman dirangkai ke query.
		config.DB.Model(&models.SensorReading{}).
			Where("created_at >= ?", since).
			Select("MIN(" + s.Code + ") AS min, AVG(" + s.Code + ") AS avg, MAX(" + s.Code + ") AS max").
			Scan(&result)

		rows = append(rows, statRow{
			Label: s.Label,
			Unit:  s.Unit,
			Min:   formatStat(result.Min, s.Decimals),
			Avg:   formatStat(result.Avg, s.Decimals),
			Max:   formatStat(result.Max, s.Decimals),
		})
	}
	return rows
}

func formatStat(v *float64, decimals int) string {
	if v == nil {
		return "-"
	}
	return strconv.FormatFloat(*v, 'f', decimals, 64)
}
