package bootstrap

import (
	"errors"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/middleware/csrf"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/session"
	"github.com/gofiber/fiber/v3/middleware/static"
	"github.com/gofiber/template/html/v3"

	"smart-greenhouse/app/http/middleware"
	"smart-greenhouse/app/services/automation"
	"smart-greenhouse/app/services/mqtt"
	"smart-greenhouse/app/support"
	"smart-greenhouse/config"
	"smart-greenhouse/database/migrations"
	"smart-greenhouse/database/seeders"
	"smart-greenhouse/routes"
)

func NewApp() *fiber.App {
	// 1. Konfigurasi & database
	config.Load()
	config.ConnectDatabase()
	migrations.Run(config.DB)
	seeders.Run(config.DB)

	// 2. Template engine (layout: resources/views/layouts/main.html)
	viewEngine := html.New("./resources/views", ".html")
	viewEngine.Reload(config.Env("APP_ENV", "local") == "local") // edit HTML tanpa restart saat development
	for name, fn := range support.ViewFuncs() {
		viewEngine.AddFunc(name, fn)
	}

	app := fiber.New(fiber.Config{
		AppName:           config.Env("APP_NAME", "Smart Greenhouse"),
		Views:             viewEngine,
		ViewsLayout:       "layouts/main",
		PassLocalsToViews: true,
		ErrorHandler:      errorHandler,
	})

	// 3. File statis (v3: lewat middleware static). Dipasang sebelum middleware lain
	// supaya CSS/JS tidak ikut membuka session.
	app.Use("/css", static.New("./public/css"))
	app.Use("/js", static.New("./public/js"))
	app.Use("/images", static.New("./public/images"))

	// 4. Middleware global (urutan penting)
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{Stream: logOutput()}))

	// Session di v3 adalah middleware: tersimpan otomatis, di handler pakai session.FromContext(c)
	app.Use(session.New(session.Config{
		CookieHTTPOnly:  true,
		CookieSameSite:  "Lax",
		IdleTimeout:     2 * time.Hour,
		AbsoluteTimeout: 24 * time.Hour,
	}))

	app.Use(csrf.New(csrf.Config{
		Extractor:      extractors.FromForm("_token"), // token dikirim lewat field form bernama _token
		CookieSameSite: "Lax",
		IdleTimeout:    24 * time.Hour,
	}))
	app.Use(middleware.ShareViewData())

	// 5. Route
	routes.Web(app)
	routes.API(app)

	// 6. MQTT + otomasi
	autoEngine := automation.NewEngine(mqtt.SetActuator)
	mqtt.OnSensor = autoEngine.Evaluate
	mqtt.Connect()

	return app
}

// logOutput menulis log request ke terminal sekaligus ke storage/logs/app.log.
func logOutput() io.Writer {
	if err := os.MkdirAll("storage/logs", 0o755); err != nil {
		return os.Stdout
	}
	file, err := os.OpenFile("storage/logs/app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Println("Tidak bisa membuka file log:", err)
		return os.Stdout
	}
	return io.MultiWriter(os.Stdout, file)
}

func errorHandler(c fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	var e *fiber.Error
	if errors.As(err, &e) {
		code = e.Code
	}

	// JavaScript / API menerima JSON, browser biasa menerima halaman error
	if c.XHR() || strings.HasPrefix(c.Path(), "/api") {
		return c.Status(code).JSON(fiber.Map{"message": err.Error()})
	}

	return c.Status(code).Render("errors/error", fiber.Map{
		"title":   "Terjadi Kesalahan",
		"appName": config.Env("APP_NAME", "Smart Greenhouse"),
		"code":    code,
		"message": err.Error(),
	})
}
