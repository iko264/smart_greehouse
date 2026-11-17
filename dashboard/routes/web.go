package routes

import (
	fiberws "github.com/gofiber/contrib/v3/websocket"
	"github.com/gofiber/fiber/v3"

	"smart-greenhouse/app/http/controllers"
	"smart-greenhouse/app/http/middleware"
	ws "smart-greenhouse/app/services/websocket"
)

func Web(app *fiber.App) {
	authC := controllers.NewAuthController()
	dashboard := controllers.NewDashboardController()
	sensor := controllers.NewSensorController()
	actuator := controllers.NewActuatorController()
	automation := controllers.NewAutomationController()

	requireLogin := middleware.Auth()
	guest := middleware.Guest()

	// Auth
	app.Get("/login", guest, authC.ShowLogin).Name("login")
	app.Post("/login", guest, authC.Login)
	app.Post("/logout", requireLogin, authC.Logout)

	// Halaman
	app.Get("/", requireLogin, dashboard.Index).Name("dashboard")
	app.Get("/monitoring", requireLogin, sensor.Monitoring).Name("monitoring")
	app.Get("/history", requireLogin, sensor.History).Name("history")

	// Kontrol aktuator
	app.Get("/actuators", requireLogin, actuator.Index).Name("actuators")
	app.Post("/actuators/:code/command", requireLogin, actuator.Command)
	app.Post("/actuators/:code/mode", requireLogin, actuator.Mode)

	// Otomasi
	app.Get("/automation", requireLogin, automation.Index).Name("automation")
	app.Post("/automation", requireLogin, automation.Store)
	app.Post("/automation/:id/toggle", requireLogin, automation.Toggle)
	app.Post("/automation/:id/delete", requireLogin, automation.Destroy)

	// WebSocket: browser menerima data sensor & status aktuator secara real-time
	app.Use("/ws", requireLogin, func(c fiber.Ctx) error {
		if fiberws.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	app.Get("/ws", ws.H.Handler())
}
