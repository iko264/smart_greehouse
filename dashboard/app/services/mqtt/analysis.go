package mqtt

import (
	"slices"
	"sync"

	"smart-greenhouse/app/models"
	"smart-greenhouse/app/support"
	"smart-greenhouse/config"
)

// SensorEvent = data sensor + hasil olahan. Field SensorReading otomatis "rata" di JSON.
type SensorEvent struct {
	models.SensorReading
	MA      map[string]float64 `json:"ma"`      // moving average per sensor
	Z       map[string]float64 `json:"z"`       // z-score per sensor
	Anomaly map[string]bool    `json:"anomaly"` // true jika |z| >= ZSCORE_THRESHOLD
}

var (
	analysisMu sync.Mutex
	history    = map[string][]float64{} // kode sensor -> nilai terakhir (lama -> baru)
)

// analyze menghitung MA & z-score tiap sensor, lalu memasukkan nilai baru ke jendela riwayat.
func analyze(r models.SensorReading) SensorEvent {
	settings := support.LoadStatsSettings()

	ev := SensorEvent{
		SensorReading: r,
		MA:            map[string]float64{},
		Z:             map[string]float64{},
		Anomaly:       map[string]bool{},
	}

	analysisMu.Lock()
	defer analysisMu.Unlock()

	for _, s := range support.Sensors {
		x := r.Value(s.Code)
		ev.MA[s.Code], ev.Z[s.Code], ev.Anomaly[s.Code] = settings.Analyze(history[s.Code], x)

		h := append(history[s.Code], x)
		if len(h) > settings.ZWindow {
			h = h[len(h)-settings.ZWindow:]
		}
		history[s.Code] = h
	}
	return ev
}

// loadHistory mengisi jendela dari database saat server baru jalan,
// supaya z-score langsung bermakna (tanpa menunggu data baru terkumpul).
func loadHistory() {
	settings := support.LoadStatsSettings()

	var rows []models.SensorReading
	config.DB.Order("id desc").Limit(settings.ZWindow).Find(&rows)
	slices.Reverse(rows)

	analysisMu.Lock()
	defer analysisMu.Unlock()

	for _, r := range rows {
		for _, s := range support.Sensors {
			history[s.Code] = append(history[s.Code], r.Value(s.Code))
		}
	}
}
