package mqtt

import (
	"fmt"
	"log"
	"strings"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"

	"smart-greenhouse/app/services/websocket"
	"smart-greenhouse/config"
)

var (
	client paho.Client
	prefix string
)

// Topic menyusun nama topic lengkap, contoh: Topic("sensors") -> "greenhouse/abc/sensors".
func Topic(parts ...string) string {
	return prefix + "/" + strings.Join(parts, "/")
}

func IsConnected() bool {
	return client != nil && client.IsConnectionOpen()
}

// Connect menyambungkan ke broker. Kalau broker mati, program tetap jalan dan mencoba lagi otomatis.
func Connect() {
	prefix = strings.Trim(config.Env("MQTT_TOPIC_PREFIX", "greenhouse/demo"), "/")
	loadHistory() // isi jendela moving average / z-score dari database

	opts := paho.NewClientOptions().
		AddBroker(config.Env("MQTT_BROKER", "tcp://broker.hivemq.com:1883")).
		// Client ID harus unik di broker, jadi ditambah timestamp.
		SetClientID(fmt.Sprintf("%s-%d", config.Env("MQTT_CLIENT_ID", "smart-greenhouse-web"), time.Now().Unix())).
		SetKeepAlive(30 * time.Second).
		SetCleanSession(true).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(5 * time.Second)

	if user := config.Env("MQTT_USERNAME", ""); user != "" {
		opts.SetUsername(user)
		opts.SetPassword(config.Env("MQTT_PASSWORD", ""))
	}

	// OnConnect jalan setiap kali tersambung (termasuk setelah reconnect), jadi subscribe dilakukan di sini.
	opts.SetOnConnectHandler(func(c paho.Client) {
		log.Println("MQTT terhubung, prefix topic:", prefix)
		subscribeAll(c)
		PublishThresholds() // kirim threshold & mode terakhir ke ESP32 (retained)
		PublishLimits()     // kirim target cahaya & batas siram terakhir (retained)
		websocket.H.Broadcast("mqtt", map[string]bool{"connected": true})
	})

	opts.SetConnectionLostHandler(func(c paho.Client, err error) {
		log.Println("MQTT terputus:", err)
		websocket.H.Broadcast("mqtt", map[string]bool{"connected": false})
	})

	client = paho.NewClient(opts)
	client.Connect() // tidak ditunggu; proses penyambungan berjalan di latar belakang
}
