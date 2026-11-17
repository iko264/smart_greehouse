package support

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/session"
	"golang.org/x/crypto/bcrypt"

	"smart-greenhouse/app/models"
	"smart-greenhouse/config"
)

// ---------------------------------------------------------------------------
// Daftar sensor (dipakai form otomasi, kartu dashboard, dan template)
// ---------------------------------------------------------------------------

type SensorInfo struct {
	Code     string
	Label    string
	Unit     string
	Icon     string // class Bootstrap Icons
	Color    string // warna Bootstrap (danger, primary, ...)
	Decimals int
}

var Sensors = []SensorInfo{
	{Code: "temperature", Label: "Suhu Udara", Unit: "°C", Icon: "bi-thermometer-half", Color: "danger", Decimals: 1},
	{Code: "humidity", Label: "Kelembapan Udara", Unit: "%", Icon: "bi-droplet", Color: "primary", Decimals: 1},
	{Code: "soil", Label: "Kelembapan Tanah", Unit: "%", Icon: "bi-moisture", Color: "success", Decimals: 0},
	{Code: "light", Label: "Intensitas Cahaya", Unit: "lux", Icon: "bi-brightness-high", Color: "warning", Decimals: 0},
}

func SensorByCode(code string) (SensorInfo, bool) {
	for _, s := range Sensors {
		if s.Code == code {
			return s, true
		}
	}
	return SensorInfo{}, false
}

// ---------------------------------------------------------------------------
// Session & flash message
// ---------------------------------------------------------------------------

// Di Fiber v3 session berupa middleware (session.New di bootstrap/app.go) yang menyimpan otomatis
// setelah handler selesai. Di handler cukup panggil session.FromContext(c).

// Flash menyimpan pesan yang hanya tampil satu kali di halaman berikutnya.
func Flash(c fiber.Ctx, key, message string) {
	if sess := session.FromContext(c); sess != nil {
		sess.Set("flash_"+key, message)
	}
}

// PullFlash mengambil pesan flash lalu menghapusnya.
func PullFlash(c fiber.Ctx, key string) string {
	sess := session.FromContext(c)
	if sess == nil {
		return ""
	}
	message, _ := sess.Get("flash_" + key).(string)
	if message != "" {
		sess.Delete("flash_" + key)
	}
	return message
}

// ---------------------------------------------------------------------------
// View & response
// ---------------------------------------------------------------------------

// View menampilkan template sekaligus menyisipkan data umum (flash message, nama aplikasi, errors).
func View(c fiber.Ctx, name string, data fiber.Map) error {
	if data == nil {
		data = fiber.Map{}
	}
	if _, ok := data["errors"]; !ok {
		data["errors"] = map[string]string{}
	}
	data["success"] = PullFlash(c, "success")
	data["error"] = PullFlash(c, "error")
	data["appName"] = config.Env("APP_NAME", "Smart Greenhouse")
	return c.Render(name, data)
}

func JSONError(c fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(fiber.Map{"message": message})
}

// ---------------------------------------------------------------------------
// Validasi (validator v10)
// ---------------------------------------------------------------------------

var validate = validator.New()

func init() {
	// Nama field di map error memakai tag `form`, supaya cocok dengan name="" di HTML.
	validate.RegisterTagNameFunc(func(f reflect.StructField) string {
		name := strings.SplitN(f.Tag.Get("form"), ",", 2)[0]
		if name == "" || name == "-" {
			return f.Name
		}
		return name
	})
}

// Validate mengembalikan map[nama_field]pesan. Map kosong berarti data valid.
func Validate(data any) map[string]string {
	errs := map[string]string{}

	err := validate.Struct(data)
	if err == nil {
		return errs
	}

	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		errs["form"] = err.Error()
		return errs
	}

	t := reflect.TypeOf(data)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	for _, e := range ve {
		label := e.Field()
		if f, ok := t.FieldByName(e.StructField()); ok {
			if l := f.Tag.Get("label"); l != "" {
				label = l
			}
		}
		errs[e.Field()] = errorMessage(label, e)
	}
	return errs
}

func errorMessage(label string, e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return label + " wajib diisi"
	case "email":
		return label + " harus berupa email yang valid"
	case "min":
		return fmt.Sprintf("%s minimal %s karakter", label, e.Param())
	case "max":
		return fmt.Sprintf("%s maksimal %s karakter", label, e.Param())
	case "gte":
		return fmt.Sprintf("%s minimal %s", label, e.Param())
	case "lte":
		return fmt.Sprintf("%s maksimal %s", label, e.Param())
	}
	return label + " tidak valid"
}

// ---------------------------------------------------------------------------
// Password
// ---------------------------------------------------------------------------

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// ---------------------------------------------------------------------------
// Fungsi tambahan untuk template (dipasang di bootstrap/app.go)
// ---------------------------------------------------------------------------

func ViewFuncs() map[string]any {
	return map[string]any{
		// {{formatTime .CreatedAt}}
		"formatTime": func(t time.Time) string {
			return t.Format("02 Jan 2006 15:04:05")
		},
		// {{active .currentPath "/monitoring"}} -> "active" jika path sama
		"active": func(current any, path string) string {
			if s, ok := current.(string); ok && s == path {
				return "active"
			}
			return ""
		},
		// {{formatValue "temperature" 28.456}} -> "28.5"
		"formatValue": func(code string, value float64) string {
			decimals := 1
			if s, ok := SensorByCode(code); ok {
				decimals = s.Decimals
			}
			return strconv.FormatFloat(value, 'f', decimals, 64)
		},
		"sensorLabel": func(code string) string {
			if s, ok := SensorByCode(code); ok {
				return s.Label
			}
			return code
		},
		// {{ruleText .}} -> "Nyala jika Suhu Udara ≥ 30 °C, mati jika ≤ 28 °C"
		"ruleText": ruleText,
		"add":      func(a, b int) int { return a + b },
	}
}

func ruleText(r models.AutomationRule) string {
	s, _ := SensorByCode(r.Sensor)

	onSign, offSign := "≥", "≤"
	if r.Direction == "below" {
		onSign, offSign = "≤", "≥"
	}

	num := func(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
	return fmt.Sprintf("Nyala jika %s %s %s %s, mati jika %s %s %s",
		s.Label, onSign, num(r.OnValue), s.Unit, offSign, num(r.OffValue), s.Unit)
}
