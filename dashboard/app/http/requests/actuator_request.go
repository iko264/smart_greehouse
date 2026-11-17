package requests

// ActuatorCommandRequest: perintah nyala/mati dari dashboard.
type ActuatorCommandRequest struct {
	State string `form:"state" json:"state" label:"Perintah" validate:"required,oneof=on off"`
}

// ActuatorModeRequest: ganti mode manual / otomatis.
type ActuatorModeRequest struct {
	Mode string `form:"mode" json:"mode" label:"Mode" validate:"required,oneof=manual auto"`
}
