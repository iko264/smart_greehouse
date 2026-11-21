package support

import (
	"math"

	"smart-greenhouse/config"
)

// StatsSettings: pengaturan moving average dan z-score (dibaca dari .env).
type StatsSettings struct {
	MAWindow  int     // jumlah data untuk moving average
	ZWindow   int     // jumlah data riwayat untuk z-score
	Threshold float64 // |z| >= Threshold dianggap anomali
}

func LoadStatsSettings() StatsSettings {
	s := StatsSettings{
		MAWindow:  config.EnvInt("MA_WINDOW", 5),
		ZWindow:   config.EnvInt("ZSCORE_WINDOW", 30),
		Threshold: config.EnvFloat("ZSCORE_THRESHOLD", 3),
	}
	if s.MAWindow < 1 {
		s.MAWindow = 1
	}
	if s.ZWindow < 5 {
		s.ZWindow = 5
	}
	return s
}

// MovingAverage: rata-rata dari x dan (window-1) nilai sebelumnya.
// history diurutkan dari yang paling lama ke yang paling baru.
func MovingAverage(history []float64, x float64, window int) float64 {
	sum, n := x, 1
	for i := len(history) - 1; i >= 0 && n < window; i-- {
		sum += history[i]
		n++
	}
	return sum / float64(n)
}

// ZScore: z = (x - rata-rata) / simpangan baku, dihitung dari history.
// Hasil 0 jika data belum cukup (< 5) atau semua nilai sama (simpangan baku 0).
func ZScore(history []float64, x float64) float64 {
	n := float64(len(history))
	if n < 5 {
		return 0
	}

	mean := 0.0
	for _, v := range history {
		mean += v
	}
	mean /= n

	variance := 0.0
	for _, v := range history {
		variance += (v - mean) * (v - mean)
	}
	std := math.Sqrt(variance / n)
	if std == 0 {
		return 0
	}
	return (x - mean) / std
}

// Analyze menghitung moving average, z-score, dan status anomali untuk nilai baru x.
// history = nilai-nilai SEBELUM x (x sendiri tidak ikut menghitung z-score).
func (s StatsSettings) Analyze(history []float64, x float64) (ma, z float64, anomaly bool) {
	if len(history) > s.ZWindow {
		history = history[len(history)-s.ZWindow:]
	}

	z = ZScore(history, x)
	ma = MovingAverage(history, x, s.MAWindow)
	anomaly = math.Abs(z) >= s.Threshold

	return round2(ma), round2(z), anomaly
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
