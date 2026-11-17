package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/csrf"
	"github.com/gofiber/fiber/v3/middleware/session"

	"smart-greenhouse/app/services/mqtt"
	"smart-greenhouse/app/support"
)

// ShareViewData menaruh data umum ke c.Locals supaya otomatis tersedia di semua template
// (karena PassLocalsToViews: true). Harus dipasang SETELAH middleware session dan csrf.
func ShareViewData() fiber.Handler {
	return func(c fiber.Ctx) error {
		if sess := session.FromContext(c); sess != nil {
			if id, ok := sess.Get("user_id").(string); ok && id != "" {
				c.Locals("userID", id)
				c.Locals("userName", sess.Get("user_name"))
			}
		}

		c.Locals("csrfToken", csrf.TokenFromContext(c))
		c.Locals("currentPath", c.Path())
		c.Locals("mqttConnected", mqtt.IsConnected())
		return c.Next()
	}
}

// Auth: hanya untuk user yang sudah login.
func Auth() fiber.Handler {
	return func(c fiber.Ctx) error {
		if c.Locals("userID") != nil {
			return c.Next()
		}

		// Request dari JavaScript / API / WebSocket dijawab 401, halaman biasa diarahkan ke login.
		path := c.Path()
		if c.XHR() || strings.HasPrefix(path, "/api") || path == "/ws" {
			return support.JSONError(c, fiber.StatusUnauthorized, "Silakan login terlebih dahulu")
		}
		return c.Redirect().To("/login")
	}
}

// Guest: hanya untuk user yang belum login (halaman login).
func Guest() fiber.Handler {
	return func(c fiber.Ctx) error {
		if c.Locals("userID") != nil {
			return c.Redirect().To("/")
		}
		return c.Next()
	}
}
