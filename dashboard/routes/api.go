package routes

import (
	"github.com/gofiber/fiber/v3"

	"smart-greenhouse/app/http/controllers"
	"smart-greenhouse/app/http/controllers/api"
	"smart-greenhouse/app/http/middleware"
)

func API(app *fiber.App) {
	sensor := api.NewSensorController()
	greenhouse := controllers.NewGreenhouseController()

	g := app.Group("/api", middleware.Auth())
	g.Get("/sensors/latest", sensor.Latest)
	g.Get("/sensors/history", sensor.History)
	g.Get("/status", sensor.Status)
	g.Get("/greenhouse/limits", greenhouse.LimitsAPI)
	g.Get("/greenhouse/daily", greenhouse.DailyAPI)
}
