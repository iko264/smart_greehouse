package controllers

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/session"

	"smart-greenhouse/app/http/requests"
	"smart-greenhouse/app/models"
	"smart-greenhouse/app/support"
	"smart-greenhouse/config"
)

type AuthController struct{}

func NewAuthController() *AuthController {
	return &AuthController{}
}

func (a *AuthController) ShowLogin(c fiber.Ctx) error {
	return support.View(c, "auth/login", fiber.Map{
		"title": "Login",
		"old":   requests.LoginRequest{},
	})
}

func (a *AuthController) Login(c fiber.Ctx) error {
	var req requests.LoginRequest
	if err := c.Bind().Form(&req); err != nil {
		return fiber.ErrBadRequest
	}

	errs := support.Validate(req)

	if len(errs) == 0 {
		var user models.User
		err := config.DB.Where("email = ?", req.Email).First(&user).Error
		if err != nil || !support.CheckPassword(user.Password, req.Password) {
			errs["email"] = "Email atau password salah"
		} else {
			sess := session.FromContext(c)
			// ID session diganti saat login untuk mencegah session fixation
			if err := sess.Regenerate(); err != nil {
				return err
			}
			sess.Set("user_id", strconv.FormatUint(uint64(user.ID), 10))
			sess.Set("user_name", user.Name)
			support.Flash(c, "success", "Selamat datang, "+user.Name+"!")
			return c.Redirect().To("/")
		}
	}

	c.Status(fiber.StatusUnprocessableEntity)
	return support.View(c, "auth/login", fiber.Map{
		"title":  "Login",
		"old":    req,
		"errors": errs,
	})
}

func (a *AuthController) Logout(c fiber.Ctx) error {
	// Reset: hapus semua data session dan ganti ID-nya
	if err := session.FromContext(c).Reset(); err != nil {
		return err
	}

	support.Flash(c, "success", "Anda sudah keluar.")
	return c.Redirect().To("/login")
}
