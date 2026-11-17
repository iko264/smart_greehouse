package requests

// AutomationRuleRequest: form tambah aturan otomasi.
type AutomationRuleRequest struct {
	Name       string  `form:"name" label:"Nama aturan" validate:"required,min=3,max=100"`
	ActuatorID uint    `form:"actuator_id" label:"Aktuator" validate:"required"`
	Sensor     string  `form:"sensor" label:"Sensor" validate:"required,oneof=temperature humidity soil light"`
	Direction  string  `form:"direction" label:"Kondisi" validate:"required,oneof=above below"`
	OnValue    float64 `form:"on_value" label:"Batas nyala" validate:"gte=-100,lte=100000"`
	OffValue   float64 `form:"off_value" label:"Batas mati" validate:"gte=-100,lte=100000"`
}

// LoginRequest: form login (dipakai AuthController).
type LoginRequest struct {
	Email    string `form:"email" label:"Email" validate:"required,email"`
	Password string `form:"password" label:"Password" validate:"required"`
}
