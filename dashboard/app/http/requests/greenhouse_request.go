package requests

// GreenhouseLimitsRequest: target jam cahaya & batas siram harian dari dashboard.
type GreenhouseLimitsRequest struct {
	LightTargetHours float64 `form:"light_target_hours" json:"light_target_hours" label:"Target jam cahaya" validate:"required,gte=1,lte=24"`
	MaxWatering      int     `form:"max_watering_per_day" json:"max_watering_per_day" label:"Batas siram harian" validate:"required,gte=1,lte=50"`
}
