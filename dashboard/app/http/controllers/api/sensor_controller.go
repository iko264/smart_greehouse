package api

import (
	"slices"

	"github.com/gofiber/fiber/v3"

	"smart-greenhouse/app/models"
	"smart-greenhouse/app/services/mqtt"
	"smart-greenhouse/app/support"
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

// historyPoint = satu titik grafik: data sensor + moving average + z-score.
type historyPoint struct {
	models.SensorReading
	MA      map[string]float64 `json:"ma"`
	Z       map[string]float64 `json:"z"`
	Anomaly map[string]bool    `json:"anomaly"`
}

// History: GET /api/sensors/history?limit=30
func (s *SensorController) History(c fiber.Ctx) error {
	limit := fiber.Query[int](c, "limit", 30)
	if limit < 1 {
		limit = 1
	}
	if limit > 500 {
		limit = 500
	}

	settings := support.LoadStatsSettings()

	// Ambil data tambahan di depan sebagai "pemanasan" jendela z-score titik pertama.
	var rows []models.SensorReading
	config.DB.Order("id desc").Limit(limit + settings.ZWindow).Find(&rows)
	slices.Reverse(rows)

	points := make([]historyPoint, 0, len(rows))
	for i, r := range rows {
		p := historyPoint{
			SensorReading: r,
			MA:            map[string]float64{},
			Z:             map[string]float64{},
			Anomaly:       map[string]bool{},
		}

		for _, sensor := range support.Sensors {
			// riwayat = nilai-nilai sebelum baris ini
			var hist []float64
			for _, prev := range rows[max(0, i-settings.ZWindow):i] {
				hist = append(hist, prev.Value(sensor.Code))
			}
			p.MA[sensor.Code], p.Z[sensor.Code], p.Anomaly[sensor.Code] =
				settings.Analyze(hist, r.Value(sensor.Code))
		}
		points = append(points, p)
	}

	if len(points) > limit {
		points = points[len(points)-limit:]
	}
	return c.JSON(fiber.Map{"data": points})
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
