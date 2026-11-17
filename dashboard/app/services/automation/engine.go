package automation

import (
	"log"
	"sync"

	"smart-greenhouse/app/models"
	"smart-greenhouse/config"
)

// Commander adalah fungsi untuk menyalakan/mematikan aktuator (diisi mqtt.SetActuator saat bootstrap).
type Commander func(code string, on bool, source string) error

type Engine struct {
	command Commander
	mu      sync.Mutex
}

func NewEngine(command Commander) *Engine {
	return &Engine{command: command}
}

// Evaluate dipanggil setiap ada data sensor baru.
// Hanya aktuator dengan mode "auto" dan aturan yang aktif yang diproses.
func (e *Engine) Evaluate(reading models.SensorReading) {
	e.mu.Lock()
	defer e.mu.Unlock()

	var rules []models.AutomationRule
	config.DB.Preload("Actuator").Where("is_active = ?", true).Find(&rules)

	for _, rule := range rules {
		if rule.Actuator.Mode != models.ModeAuto {
			continue
		}

		turnOn, changed := decide(rule, reading.Value(rule.Sensor), rule.Actuator.State)
		if !changed {
			continue
		}

		if err := e.command(rule.Actuator.Code, turnOn, "auto"); err != nil {
			log.Printf("[otomasi] aturan %q gagal: %v", rule.Name, err)
		}
	}
}

// decide menerapkan histeresis. Mengembalikan (nyala?, apakah_berubah?).
//
// Contoh kipas (above, on=30, off=28):
//
//	suhu 31 -> NYALA | suhu 29 -> tetap nyala | suhu 27.9 -> MATI | suhu 29 -> tetap mati
func decide(rule models.AutomationRule, value float64, isOn bool) (turnOn bool, changed bool) {
	switch rule.Direction {
	case "above":
		if !isOn && value >= rule.OnValue {
			return true, true
		}
		if isOn && value <= rule.OffValue {
			return false, true
		}
	case "below":
		if !isOn && value <= rule.OnValue {
			return true, true
		}
		if isOn && value >= rule.OffValue {
			return false, true
		}
	}
	return isOn, false
}
