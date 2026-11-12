#include <WiFi.h>
#include <PubSubClient.h>
#include <DHT.h>
#include <ArduinoJson.h>

// WIFI WOKWI
const char* WIFI_SSID = "Wokwi-GUEST";
const char* WIFI_PASSWORD = "";
// MQTT
const char* MQTT_SERVER = "broker.hivemq.com";
const int MQTT_PORT = 1883;
// MQTT TOPIC
#define TOPIC_SENSOR "smartgreenhouse/kelompok678pemancinghandal123/sensor"
#define TOPIC_STATUS "smartgreenhouse/kelompok678pemancinghandal123/status"
// PIN
#define DHTPIN 4
#define DHTTYPE DHT22

#define SOIL_PIN 34
#define LDR_PIN 35

#define FAN_PIN 18
#define PUMP_PIN 19
#define LIGHT_PIN 21

// OBJECT
DHT dht(DHTPIN, DHTTYPE);

WiFiClient espClient;
PubSubClient mqttClient(espClient);

// THRESHOLD
// FAN
#define TEMP_ON 30.0
#define TEMP_OFF 28.0
// PUMP
#define SOIL_ON 30
#define SOIL_OFF 40
// GROW LIGHT
#define LIGHT_ON 30
#define LIGHT_OFF 40

// STATUS
bool fanState = false;
bool pumpState = false;
bool lightState = false;

// WIFI
void setupWiFi() {  
  Serial.print("Connecting to WiFi");
  WiFi.begin(WIFI_SSID, WIFI_PASSWORD, 6);
  while (WiFi.status() != WL_CONNECTED) {
    delay(500);
    Serial.print(".");
  }
  Serial.println();
  Serial.println("WiFi Connected!");
  Serial.print("IP Address: ");
  Serial.println(WiFi.localIP());
}

// MQTT CONNECT
void reconnectMQTT() {
  while (!mqttClient.connected()) {
    Serial.print("Connecting to MQTT...");
    String clientID =
      "ESP32-Greenhouse-" +
      String(random(0xffff), HEX);
    if (mqttClient.connect(clientID.c_str())) {
      Serial.println("CONNECTED!");
    }
    else {
      Serial.print("FAILED, rc=");
      Serial.print(mqttClient.state());
      Serial.println(" retrying...");
      delay(2000);
    }
  }
}

// SETUP
void setup() {
  Serial.begin(115200);
  // DHT22
  dht.begin();
  // Pin actuator
  pinMode(FAN_PIN, OUTPUT);
  pinMode(PUMP_PIN, OUTPUT);
  pinMode(LIGHT_PIN, OUTPUT);
  // Kondisi awal actuator OFF
  digitalWrite(FAN_PIN, LOW);
  digitalWrite(PUMP_PIN, LOW);
  digitalWrite(LIGHT_PIN, LOW);

  setupWiFi();
  mqttClient.setServer(MQTT_SERVER, MQTT_PORT);
  mqttClient.setBufferSize(512);

  Serial.println();
  Serial.println("================================");
  Serial.println("      SMART GREENHOUSE ESP32");
  Serial.println("================================");
}

// LOOP
void loop() {
  if (!mqttClient.connected()) {
    reconnectMQTT();
  }
  mqttClient.loop();
  float temperature = dht.readTemperature();
  float humidity = dht.readHumidity();

  int soilRaw = analogRead(SOIL_PIN);
  int lightRaw = analogRead(LDR_PIN);

  if (
    isnan(temperature) ||
    isnan(humidity)
  ) {
    Serial.println("Gagal membaca DHT22!");
    delay(2000);
    return;
  }

  // KONVERSI SOIL & KONVERSI CAHAYA
  int soilPercent = map(soilRaw, 0, 4095, 0, 100);
  soilPercent = constrain( soilPercent, 0, 100);
  int lightPercent = map(lightRaw, 0, 4095, 100, 0);
  lightPercent = constrain(lightPercent, 0, 100);

  // KONTROL FAN
  if (!fanState && temperature > TEMP_ON) {
    fanState = true;
  }
  if (fanState && temperature <= TEMP_OFF) {
    fanState = false;
  }
  digitalWrite(FAN_PIN, fanState);

  // KONTROL POMPA
  if (!pumpState && soilPercent < SOIL_ON) {
    pumpState = true;
  }
  if (pumpState && soilPercent >= SOIL_OFF) {
    pumpState = false;}

  digitalWrite(PUMP_PIN, pumpState);

  // KONTROL GROW LIGHT
  if (!lightState && lightPercent < LIGHT_ON) {
    lightState = true;
  }
  if (lightState && lightPercent >= LIGHT_OFF) {
    lightState = false;
  }
  digitalWrite(LIGHT_PIN, lightState);

  // BUAT JSON SENSOR
  StaticJsonDocument<256> sensorDoc;
  sensorDoc["temperature"] =temperature;
  sensorDoc["humidity"] = humidity;
  sensorDoc["soil"] = soilPercent;
  sensorDoc["light"] = lightPercent;
  char sensorJson[160]; // Buffer JSON sensor
  serializeJson(sensorDoc, sensorJson, sizeof(sensorJson));
  // BUAT JSON STATUS
  StaticJsonDocument<256> statusDoc;
  statusDoc["fan"] = fanState ? "ON" : "OFF";
  statusDoc["pump"] = pumpState ? "ON" : "OFF";
  statusDoc["grow"] = lightState ? "ON" : "OFF";
  char statusJson[100]; // Buffer JSON status
  serializeJson(statusDoc, statusJson, sizeof(statusJson));

  // MQTT PUBLISH
  mqttClient.publish(TOPIC_SENSOR, sensorJson, true);
  mqttClient.publish(TOPIC_STATUS, statusJson, true);

  Serial.println();
  Serial.println("------ SMART GREENHOUSE ------");
  Serial.println("Sensor:");
  Serial.print("  temperature: ");
  Serial.println(temperature, 2);
  Serial.print("  humidity: ");
  Serial.println(humidity, 2);
  Serial.print("  soil: ");
  Serial.println(soilPercent);
  Serial.print("  light: ");
  Serial.println(lightPercent);
  Serial.println();
  Serial.println("Status:");
  Serial.print("  fan: ");
  Serial.println(fanState ? "ON" : "OFF");
  Serial.print("  pump: ");
  Serial.println(pumpState ? "ON" : "OFF");
  Serial.print("  grow_light: ");
  Serial.println(lightState ? "ON" : "OFF");
  Serial.println("-------------------------------");
  delay(2000);
}