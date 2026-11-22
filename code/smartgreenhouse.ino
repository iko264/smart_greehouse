#include <ESP8266WiFi.h>
#include <PubSubClient.h>
#include <ArduinoJson.h>
#include <Wire.h>
#include <LiquidCrystal_I2C.h>
#include <DHT.h>
#include <EEPROM.h>
#include <time.h>

const char* WIFI_SSID = "";
const char* WIFI_PASS = "";

const char* MQTT_HOST   = "broker.hivemq.com";
const int   MQTT_PORT   = 1883;
const char* MQTT_PREFIX = "smartgreenhouse/kelompok678pemancinghandal123";

#define SOIL_SENSOR  A0
#define SOIL_POWER   D8
#define RELAY_POMPA  D5
#define RELAY_LAMPU  D4
#define RELAY_KIPAS  D7
#define RELAY_KIPAS2 D6
#define DHT_PIN      D3
#define DHT_TIPE     DHT11
#define LDR_DO       D0
#define SDA_PIN      D2
#define SCL_PIN      D1

#define RELAY_ON  LOW
#define RELAY_OFF HIGH

#define LDR_TERANG_STATE LOW

const float LUX_TERANG = 650.0;
const float LUX_GELAP  = 15.0;

const int ADC_KERING = 1023;
const int ADC_BASAH  = 467;

const unsigned long DURASI_POMPA_MS = 5000UL;
const unsigned long JEDA_SIRAM_MS   = 30UL * 1000UL;
const bool TES_POMPA_SAAT_BOOT = false;

const float KIPAS_TURUN_MIN = 1.0;
const unsigned long KIPAS2_TUNGGU_MS = 10UL * 60UL * 1000UL;
const unsigned long KIPAS_CEK_PERCUMA_MS = 15UL * 60UL * 1000UL;
const unsigned long KIPAS_ISTIRAHAT_MS   = 10UL * 60UL * 1000UL;

const unsigned long JAM_LAMPU_MULAI   = 6;
const unsigned long JAM_LAMPU_SELESAI = 22;
const unsigned long KONFIRMASI_LDR_MS = 30000UL;
const unsigned long MIN_LAMPU_NYALA_MS = 10UL * 60UL * 1000UL;

unsigned long targetLightDuration = 16UL * 3600UL;
int maksSiramHariIni = 6;

const unsigned long INTERVAL_BACA_TANAH  = 2000UL;
const unsigned long INTERVAL_DHT         = 2000UL;
const unsigned long INTERVAL_LCD         = 1000UL;
const unsigned long INTERVAL_WIFI        = 15000UL;
const unsigned long INTERVAL_MQTT_COBA   = 5000UL;
const unsigned long INTERVAL_KIRIM_SENSOR = 5000UL;
const unsigned long INTERVAL_KIRIM_HARIAN = 30000UL;
const unsigned long INTERVAL_SIMPan = 30UL * 60UL * 1000UL;

struct Threshold {
  bool  aktif;
  float onValue;
  float offValue;
};

Threshold thPompa = { false, 30.0, 45.0 };
Threshold thKipas = { false, 32.0, 30.0 };
Threshold thLampu = { false, 300.0, 500.0 };

LiquidCrystal_I2C lcd(0x27, 16, 2);
DHT dht(DHT_PIN, DHT_TIPE);
WiFiClient wifiMqtt;
PubSubClient mqtt(wifiMqtt);

enum StatusTanah { TANAH_AMAN, TANAH_KERING };

struct DataSimpan {
  uint32_t magic;
  int32_t kunciHari;
  uint32_t totalCahaya;
  int32_t jumlahSiram;
};

const uint32_t MAGIC_DATA = 0xA5C31B02;

StatusTanah statusTanah = TANAH_AMAN;

int adcValue = 0;
int moisture = 0;

float suhu = NAN;
float kelembapanUdara = NAN;
bool dhtValid = false;
float suhuKirim = 0.0;
float rhKirim = 0.0;

bool pompaOn = false;
bool lampuOn = false;
bool kipasOn = false;
bool kipas2On = false;
bool pompaAutoSiram = false;

unsigned long tKipasMulai = 0, tKipas2Mulai = 0;
unsigned long tIstirahatKipas = 0;
float suhuAwalKipas = NAN, suhuAwalKipas2 = NAN;

unsigned long tPompa = 0;
unsigned long tSelesaiSiram = 0;
unsigned long tLampuNyala = 0;
bool sudahPernahSiram = false;
int jumlahSiramHariIni = 0;

unsigned long totalLightDuration = 0;
bool ldrTerangStabil = false;
bool ldrKandidat = false;
unsigned long tLdrKandidat = 0;

int kunciHariTercatat = 0;
uint32_t totalTerakhirDisimpan = 0;
int siramTerakhirDisimpan = 0;

unsigned long tTick = 0;
unsigned long tBacaTanah = 0;
unsigned long tBacaDHT = 0;
unsigned long tLcd = 0;
unsigned long tSimpan = 0;
unsigned long tWifiCoba = 0;
unsigned long tMqttCoba = 0;
unsigned long tKirimSensor = 0;
unsigned long tKirimHarian = 0;
unsigned long tHalamanLcd = 0;
int halamanLcd = 0;

void setRelay(uint8_t pin, bool nyala) {
  digitalWrite(pin, nyala ? RELAY_ON : RELAY_OFF);
}

String stateText(bool on) {
  return on ? "ON" : "OFF";
}

String formatDurasi(unsigned long detik) {
  return String(detik / 3600UL) + "j " +
         String((detik % 3600UL) / 60UL) + "m";
}

String topikSensor() {
  return String(MQTT_PREFIX) + "/sensors";
}

String topikSet(const char* kode) {
  return String(MQTT_PREFIX) + "/actuators/" + kode + "/set";
}

String topikState(const char* kode) {
  return String(MQTT_PREFIX) + "/actuators/" + kode + "/state";
}

String topikThreshold() {
  return String(MQTT_PREFIX) + "/config/thresholds";
}

String topikLimits() {
  return String(MQTT_PREFIX) + "/config/limits";
}

String topikStatusHarian() {
  return String(MQTT_PREFIX) + "/status/daily";
}

void kirimState(const char* kode, bool nyala) {
  if (!mqtt.connected()) return;
  mqtt.publish(topikState(kode).c_str(), nyala ? "ON" : "OFF");
}

void kirimSemuaState() {
  kirimState("pump", pompaOn);
  kirimState("fan", kipasOn);
  kirimState("light", lampuOn);
}

void kirimSensor() {
  if (!mqtt.connected()) return;

  char payload[128];
  snprintf(payload, sizeof(payload),
           "{\"temperature\":%.1f,\"humidity\":%.1f,\"soil\":%d,\"light\":%.0f}",
           suhuKirim, rhKirim, moisture,
           ldrTerangStabil ? LUX_TERANG : LUX_GELAP);

  mqtt.publish(topikSensor().c_str(), payload);
}

void kirimStatusHarian() {
  if (!mqtt.connected()) return;

  char payload[64];
  snprintf(payload, sizeof(payload),
           "{\"light_seconds\":%lu,\"water_count\":%d}",
           totalLightDuration, jumlahSiramHariIni);

  mqtt.publish(topikStatusHarian().c_str(), payload);
}

void sambungkanMqtt() {
  String clientId = "greenhouse-" + String(ESP.getChipId(), HEX);

  Serial.print("Menghubungkan MQTT ");
  Serial.print(MQTT_HOST);
  Serial.print(" ... ");

  if (!mqtt.connect(clientId.c_str())) {
    Serial.println("gagal (" + String(mqtt.state()) + ")");
    return;
  }
  Serial.println("terhubung");

  mqtt.subscribe(topikSet("pump").c_str(), 1);
  mqtt.subscribe(topikSet("fan").c_str(), 1);
  mqtt.subscribe(topikSet("light").c_str(), 1);
  mqtt.subscribe(topikThreshold().c_str(), 1);
  mqtt.subscribe(topikLimits().c_str(), 1);

  kirimSemuaState();
  kirimSensor();
  kirimStatusHarian();
}

void jagaMqtt() {
  if (mqtt.connected()) {
    mqtt.loop();
    return;
  }

  if (!wifiOk()) return;

  if (millis() - tMqttCoba >= INTERVAL_MQTT_COBA) {
    tMqttCoba = millis();
    sambungkanMqtt();
  }
}

bool pesanOn(const String& pesan) {
  String p = pesan;
  p.trim();
  p.toUpperCase();
  return (p == "ON" || p == "1" || p == "TRUE");
}

void pompaSet(bool nyala, bool hitungSiram) {
  if (nyala == pompaOn) return;

  pompaOn = nyala;

  if (nyala) {
    pompaAutoSiram = hitungSiram;
    tPompa = millis();

    if (hitungSiram) {
      jumlahSiramHariIni++;
      simpanData();
      Serial.print("POMPA OTOMATIS (OFFLINE) ON - siram ke-");
      Serial.println(jumlahSiramHariIni);
    } else {
      Serial.println("POMPA ON (perintah dashboard)");
    }
  } else {
    pompaAutoSiram = false;
    tSelesaiSiram = millis();
    sudahPernahSiram = true;
    Serial.println("POMPA OFF");
  }

  setRelay(RELAY_POMPA, nyala);
  kirimState("pump", nyala);
  kirimStatusHarian();
}

void kipasSet(bool nyala) {
  if (nyala == kipasOn) return;

  kipasOn = nyala;
  setRelay(RELAY_KIPAS, nyala);
  kirimState("fan", nyala);

  Serial.print("KIPAS 1 ");
  Serial.println(stateText(nyala));
}

void kipas2Set(bool nyala) {
  if (nyala == kipas2On) return;

  kipas2On = nyala;
  setRelay(RELAY_KIPAS2, nyala);

  Serial.print("KIPAS 2 ");
  Serial.println(stateText(nyala));
}

void lampuSet(bool nyala, const char* alasan) {
  if (nyala == lampuOn) return;

  lampuOn = nyala;
  setRelay(RELAY_LAMPU, nyala);
  kirimState("light", nyala);

  if (nyala) tLampuNyala = millis();

  Serial.print("GROW LIGHT ");
  Serial.print(nyala ? "ON" : "OFF");
  Serial.print(" - ");
  Serial.println(alasan);
}

void terimaThreshold(const String& payload) {
  DynamicJsonDocument doc(1024);
  DeserializationError err = deserializeJson(doc, payload);
  if (err) {
    Serial.print("Threshold JSON tidak valid: ");
    Serial.println(err.c_str());
    return;
  }

  JsonObject p = doc["pump"];
  if (!p.isNull()) {
    thPompa.aktif    = p["active"] | false;
    thPompa.onValue  = p["on_value"] | thPompa.onValue;
    thPompa.offValue = p["off_value"] | thPompa.offValue;
  }

  JsonObject f = doc["fan"];
  if (!f.isNull()) {
    thKipas.aktif    = f["active"] | false;
    thKipas.onValue  = f["on_value"] | thKipas.onValue;
    thKipas.offValue = f["off_value"] | thKipas.offValue;
  }

  JsonObject l = doc["light"];
  if (!l.isNull()) {
    thLampu.aktif    = l["active"] | false;
    thLampu.onValue  = l["on_value"] | thLampu.onValue;
    thLampu.offValue = l["off_value"] | thLampu.offValue;
  }

  Serial.print("Threshold diterima | P: ");
  Serial.print(thPompa.onValue); Serial.print("/"); Serial.print(thPompa.offValue);
  Serial.print(" | F: ");
  Serial.print(thKipas.onValue); Serial.print("/"); Serial.print(thKipas.offValue);
  Serial.print(" | L: ");
  Serial.print(thLampu.onValue); Serial.print("/"); Serial.println(thLampu.offValue);
}

void terimaLimits(const String& payload) {
  DynamicJsonDocument doc(256);
  DeserializationError err = deserializeJson(doc, payload);
  if (err) {
    Serial.print("Limits JSON tidak valid: ");
    Serial.println(err.c_str());
    return;
  }

  float jamTarget = doc["light_target_hours"] | (targetLightDuration / 3600.0);
  int siramMax = doc["max_watering_per_day"] | maksSiramHariIni;

  if (jamTarget >= 1.0 && jamTarget <= 24.0) {
    targetLightDuration = (unsigned long)(jamTarget * 3600.0);
  }
  if (siramMax >= 1 && siramMax <= 50) {
    maksSiramHariIni = siramMax;
  }

  Serial.print("Limits diterima | Target cahaya: ");
  Serial.print(targetLightDuration / 3600.0);
  Serial.print(" jam | Batas siram: ");
  Serial.println(maksSiramHariIni);
}

void mqttCallback(char* topic, byte* payload, unsigned int length) {
  String pesan;
  pesan.reserve(length);
  for (unsigned int i = 0; i < length; i++) pesan += (char)payload[i];

  String t = String(topic);

  Serial.print("MQTT [");
  Serial.print(t);
  Serial.print("] ");
  Serial.println(pesan);

  if (t == topikSet("pump")) {
    bool on = pesanOn(pesan);
    pompaSet(on, on && thPompa.aktif);
  } else if (t == topikSet("fan")) {
    bool on = pesanOn(pesan);
    kipasSet(on);
    if (!on) kipas2Set(false);
  } else if (t == topikSet("light")) {
    lampuSet(pesanOn(pesan), "perintah dashboard");
  } else if (t == topikThreshold()) {
    terimaThreshold(pesan);
  } else if (t == topikLimits()) {
    terimaLimits(pesan);
  }
}

bool wifiOk() {
  return WiFi.status() == WL_CONNECTED;
}

void jagaWiFi() {
  if (wifiOk()) return;

  if (millis() - tWifiCoba >= INTERVAL_WIFI) {
    tWifiCoba = millis();
    Serial.println("WiFi terputus, mencoba reconnect...");
    WiFi.reconnect();
  }
}

bool ambilWaktu(int &jam, int &kunci) {
  time_t now = time(nullptr);
  if (now < 100000) return false;

  struct tm waktu;
  localtime_r(&now, &waktu);

  jam = waktu.tm_hour;
  kunci = (waktu.tm_year + 1900) * 1000 + waktu.tm_yday;
  return true;
}

String jamSekarang() {
  time_t now = time(nullptr);
  if (now < 100000) return "--:--";

  struct tm waktu;
  localtime_r(&now, &waktu);

  char buffer[6];
  sprintf(buffer, "%02d:%02d", waktu.tm_hour, waktu.tm_min);
  return String(buffer);
}

void simpanData() {
  DataSimpan data;
  data.magic = MAGIC_DATA;
  data.kunciHari = kunciHariTercatat;
  data.totalCahaya = totalLightDuration;
  data.jumlahSiram = jumlahSiramHariIni;

  EEPROM.put(0, data);
  EEPROM.commit();

  totalTerakhirDisimpan = totalLightDuration;
  siramTerakhirDisimpan = jumlahSiramHariIni;
}

void muatData() {
  DataSimpan data;
  EEPROM.get(0, data);

  if (data.magic == MAGIC_DATA) {
    kunciHariTercatat = data.kunciHari;
    totalLightDuration = data.totalCahaya;
    jumlahSiramHariIni = data.jumlahSiram;
    totalTerakhirDisimpan = data.totalCahaya;
    siramTerakhirDisimpan = data.jumlahSiram;

    Serial.print("Data EEPROM dimuat. Siram hari ini: ");
    Serial.println(jumlahSiramHariIni);
  } else {
    Serial.println("Data EEPROM kosong/baru: counter mulai dari 0.");
  }
}

void cekGantiHari() {
  int jam, kunci;
  if (!ambilWaktu(jam, kunci)) return;
  if (kunci == kunciHariTercatat) return;

  totalLightDuration = 0;
  jumlahSiramHariIni = 0;
  kunciHariTercatat = kunci;
  simpanData();

  Serial.println("Hari baru: counter cahaya dan siram di-reset.");
}

int bacaSoilRataRata(int jumlahSampel = 8) {
  digitalWrite(SOIL_POWER, HIGH);
  delay(300);

  long total = 0;
  for (int i = 0; i < jumlahSampel; i++) {
    total += analogRead(SOIL_SENSOR);
    delay(5);
  }

  digitalWrite(SOIL_POWER, LOW);
  return (int)(total / jumlahSampel);
}

void bacaTanah() {
  adcValue = bacaSoilRataRata();

  moisture = map(adcValue, ADC_KERING, ADC_BASAH, 0, 100);
  moisture = constrain(moisture, 0, 100);

  Serial.print("ADC: ");
  Serial.print(adcValue);
  Serial.print(" | Tanah: ");
  Serial.print(moisture);
  Serial.print("% | Suhu: ");
  Serial.print(suhu);
  Serial.print(" | RH: ");
  Serial.print(kelembapanUdara);
  Serial.print(" | LDR raw: ");
  Serial.print(digitalRead(LDR_DO));
  Serial.print(" | LDR: ");
  Serial.print(ldrTerangStabil ? "TERANG" : "GELAP");
  Serial.print(" | Status tanah: ");
  Serial.print(statusTanah == TANAH_KERING ? "KERING" : "AMAN");
  Serial.print(" | Siram: ");
  Serial.print(jumlahSiramHariIni);
  Serial.print("/");
  Serial.print(maksSiramHariIni);
  Serial.print(" | Pompa: ");
  Serial.println(stateText(pompaOn));
}

void urusPompa() {
  if (pompaOn && pompaAutoSiram && millis() - tPompa >= DURASI_POMPA_MS) {
    pompaSet(false, false);
    Serial.println("POMPA OFF - menunggu air meresap.");
  }
}

void logikaTanahAuto() {
  if (moisture <= thPompa.onValue && statusTanah != TANAH_KERING) {
    statusTanah = TANAH_KERING;
    Serial.println("STATUS TANAH: KERING");
  }

  if (moisture >= thPompa.offValue && statusTanah != TANAH_AMAN) {
    statusTanah = TANAH_AMAN;
    Serial.println("STATUS TANAH: SUDAH LEMBAP - penyiraman berhenti.");
  }

  if (statusTanah == TANAH_KERING && !pompaOn) {
    bool jedaCukup =
      !sudahPernahSiram ||
      (millis() - tSelesaiSiram >= JEDA_SIRAM_MS);

    if (jedaCukup) {
      if (jumlahSiramHariIni < maksSiramHariIni) {
        pompaSet(true, true);
      } else {
        Serial.println("BATAS SIRAM HARIAN TERCAPAI. Periksa air/pompa/sensor.");
      }
    }
  }
}

void bacaDHT() {
  float h = dht.readHumidity();
  float t = dht.readTemperature();

  if (isnan(h) || isnan(t)) {
    dhtValid = false;
    Serial.println("Gagal membaca DHT11. Periksa kabel DATA D3.");
    return;
  }

  suhu = t;
  kelembapanUdara = h;
  dhtValid = true;
  suhuKirim = t;
  rhKirim = h;

  Serial.print("DHT11 | Suhu: ");
  Serial.print(suhu);
  Serial.print(" C | Kelembapan: ");
  Serial.print(kelembapanUdara);
  Serial.println(" %");
}

bool kipasIstirahat() {
  return (tIstirahatKipas != 0) &&
         (millis() - tIstirahatKipas < KIPAS_ISTIRAHAT_MS);
}

void logikaKipasOffline() {
  if (!dhtValid || kipasIstirahat()) {
    kipasSet(false);
    return;
  }

  bool minta = kipasOn
    ? (suhu > thKipas.offValue)
    : (suhu >= thKipas.onValue);

  if (minta && !kipasOn) {
    tKipasMulai = millis();
    suhuAwalKipas = suhu;
  }

  kipasSet(minta);
}

void logikaKipas2Offline() {
  if (!dhtValid || !kipasOn || kipasIstirahat()) {
    kipas2Set(false);
    return;
  }

  bool minta = false;

  if (!kipas2On) {
    if (millis() - tKipasMulai >= KIPAS2_TUNGGU_MS) {
      if (suhu > suhuAwalKipas - KIPAS_TURUN_MIN) {
        minta = true;
      } else {
        tKipasMulai = millis();
        suhuAwalKipas = suhu;
      }
    }
  } else {
    minta = true;

    if (millis() - tKipas2Mulai >= KIPAS_CEK_PERCUMA_MS) {
      if (suhu > suhuAwalKipas2 - KIPAS_TURUN_MIN) {
        minta = false;
        tIstirahatKipas = millis();
        Serial.println("Kipas 1 dan 2 tidak efektif (suhu tidak turun) - istirahat 10 menit.");
      } else {
        tKipas2Mulai = millis();
        suhuAwalKipas2 = suhu;
      }
    }
  }

  if (minta && !kipas2On) {
    tKipas2Mulai = millis();
    suhuAwalKipas2 = suhu;
  }

  kipas2Set(minta);
}

void bacaLDR() {
  bool rawTerang = (digitalRead(LDR_DO) == LDR_TERANG_STATE);

  if (rawTerang != ldrKandidat) {
    ldrKandidat = rawTerang;
    tLdrKandidat = millis();
  }

  if (rawTerang != ldrTerangStabil &&
      millis() - tLdrKandidat >= KONFIRMASI_LDR_MS) {
    ldrTerangStabil = rawTerang;

    Serial.print("LDR berubah: ");
    Serial.println(ldrTerangStabil ? "TERANG" : "GELAP");
  }
}

void logikaLampuOffline() {
  bool minta = false;
  const char* alasan = "otomatis offline";

  int jam, kunci;
  bool waktuValid = ambilWaktu(jam, kunci);

  if (!waktuValid) {
    minta = !ldrTerangStabil;
    alasan = "LDR (jam belum sinkron)";
  } else {
    bool dalamJendela =
      (jam >= JAM_LAMPU_MULAI && jam < JAM_LAMPU_SELESAI);
    bool targetTercapai =
      (totalLightDuration >= targetLightDuration);

    if (!dalamJendela) {
      minta = false;
      alasan = "di luar jam 06:00-22:00";
    } else if (targetTercapai) {
      minta = false;
      alasan = "target cahaya harian tercapai";
    } else if (!lampuOn) {
      minta = !ldrTerangStabil;
      alasan = minta ? "LDR mendeteksi gelap" : "LDR mendeteksi terang";
    } else if (millis() - tLampuNyala < MIN_LAMPU_NYALA_MS) {
      minta = true;
      alasan = "durasi minimum 10 menit";
    } else {
      minta = !ldrTerangStabil;
      alasan = minta ? "LDR mendeteksi gelap" : "LDR mendeteksi terang";
    }
  }

  lampuSet(minta, alasan);
}

void tickDetik() {
  cekGantiHari();

  if (lampuOn || ldrTerangStabil) {
    totalLightDuration++;
  }

  if (!mqtt.connected()) {
    logikaLampuOffline();
    logikaKipasOffline();
    logikaKipas2Offline();
  }
}

void lcdBaris(int baris, String teks) {
  while (teks.length() < 16) teks += ' ';
  if (teks.length() > 16) teks = teks.substring(0, 16);

  lcd.setCursor(0, baris);
  lcd.print(teks);
}

void tampilLcd() {
  if (millis() - tHalamanLcd >= 3000UL) {
    tHalamanLcd = millis();
    halamanLcd = (halamanLcd + 1) % 4;
  }

  if (halamanLcd == 0) {
    String t = dhtValid ? String((int)round(suhu)) : "--";
    String h = dhtValid ? String((int)round(kelembapanUdara)) : "--";

    lcdBaris(0, "T:" + t + "C H:" + h + "%");
    lcdBaris(1, "Tanah:" + String(moisture) + "% " +
              (statusTanah == TANAH_KERING ? "KERING" : "AMAN"));
  } else if (halamanLcd == 1) {
    lcdBaris(0, String("Net:") +
              (mqtt.connected() ? "MQTT OK" : (wifiOk() ? "MQTT --" : "OFFLINE")));
    lcdBaris(1, "Cahaya total:" + formatDurasi(totalLightDuration));
  } else if (halamanLcd == 2) {
    lcdBaris(0, "P:" + stateText(pompaOn) +
              " L:" + stateText(lampuOn) +
              " S:" + String(jumlahSiramHariIni));
    lcdBaris(1, "K1:" + stateText(kipasOn) +
              " K2:" + stateText(kipas2On));
  } else {
    lcdBaris(0, "P:" + String((int)thPompa.onValue) + "/" +
              String((int)thPompa.offValue) +
              " F:" + String((int)thKipas.onValue) + "/" +
              String((int)thKipas.offValue));
    lcdBaris(1, "Cah:" + String(targetLightDuration / 3600UL) + "j" +
              " Sir:" + String(maksSiramHariIni) + "x");
  }
}

void setup() {
  Serial.begin(115200);
  Serial.println();
  Serial.println("=== SMART GREENHOUSE START ===");

  digitalWrite(RELAY_POMPA, RELAY_OFF);
  digitalWrite(RELAY_LAMPU, RELAY_OFF);
  digitalWrite(RELAY_KIPAS, RELAY_OFF);
  digitalWrite(RELAY_KIPAS2, RELAY_OFF);

  pinMode(RELAY_POMPA, OUTPUT);
  pinMode(RELAY_LAMPU, OUTPUT);
  pinMode(RELAY_KIPAS, OUTPUT);
  pinMode(RELAY_KIPAS2, OUTPUT);

  digitalWrite(RELAY_POMPA, RELAY_OFF);
  digitalWrite(RELAY_LAMPU, RELAY_OFF);
  digitalWrite(RELAY_KIPAS, RELAY_OFF);
  digitalWrite(RELAY_KIPAS2, RELAY_OFF);

  pinMode(SOIL_POWER, OUTPUT);
  digitalWrite(SOIL_POWER, LOW);

  if (TES_POMPA_SAAT_BOOT) {
    Serial.println("TES POMPA: relay ON 2 detik");
    setRelay(RELAY_POMPA, true);
    delay(2000);
    setRelay(RELAY_POMPA, false);
    Serial.println("TES POMPA: selesai");
  }

  pinMode(LDR_DO, INPUT);
  ldrKandidat = (digitalRead(LDR_DO) == LDR_TERANG_STATE);
  ldrTerangStabil = ldrKandidat;
  tLdrKandidat = millis();

  dht.begin();

  EEPROM.begin(32);
  muatData();

  Wire.begin(SDA_PIN, SCL_PIN);
  lcd.init();
  lcd.backlight();
  lcd.clear();
  lcdBaris(0, "Smart Greenhouse");
  lcdBaris(1, "Starting system..");
  delay(1000);

  WiFi.mode(WIFI_STA);
  WiFi.begin(WIFI_SSID, WIFI_PASS);

  lcd.clear();
  lcdBaris(0, "Connecting WiFi");
  lcdBaris(1, "Please wait...");
  Serial.print("Menghubungkan WiFi");

  unsigned long mulai = millis();
  unsigned long tAnimasi = 0;
  int titik = 0;

  while (WiFi.status() != WL_CONNECTED &&
         millis() - mulai < 20000UL) {
    if (millis() - tAnimasi >= 500UL) {
      tAnimasi = millis();
      titik = (titik + 1) % 4;

      String animasi = "Please wait";
      for (int i = 0; i < titik; i++) animasi += ".";

      lcdBaris(1, animasi);
      Serial.print(".");
    }
    delay(10);
  }
  Serial.println();

  if (wifiOk()) {
    Serial.print("WiFi OK, IP: ");
    Serial.println(WiFi.localIP());

    lcd.clear();
    lcdBaris(0, "WiFi Connected!");
    lcdBaris(1, WiFi.localIP().toString());

    configTime(7 * 3600, 0, "pool.ntp.org", "time.nist.gov");

    mqtt.setServer(MQTT_HOST, MQTT_PORT);
    mqtt.setCallback(mqttCallback);
    mqtt.setBufferSize(512);
    sambungkanMqtt();
  } else {
    Serial.println("WiFi gagal. Sistem tetap berjalan (mode offline).");

    lcd.clear();
    lcdBaris(0, "WiFi Failed");
    lcdBaris(1, "Offline Mode");
  }

  delay(2000);
  lcd.clear();

  unsigned long now = millis();
  tTick = now;
  tBacaTanah = now;
  tBacaDHT = now - INTERVAL_DHT;
  tLcd = now;
  tSimpan = now;
  tHalamanLcd = now;
  tKirimSensor = now;
  tKirimHarian = now;
  tMqttCoba = now;

  Serial.println("Sistem siap.");
}

void loop() {
  jagaWiFi();
  jagaMqtt();
  urusPompa();
  bacaLDR();

  unsigned long now = millis();

  while (millis() - tTick >= 1000UL) {
    tTick += 1000UL;
    tickDetik();
  }

  if (now - tBacaDHT >= INTERVAL_DHT) {
    tBacaDHT = now;
    bacaDHT();
  }

  if (now - tBacaTanah >= INTERVAL_BACA_TANAH) {
    tBacaTanah = now;
    bacaTanah();

    if (thPompa.aktif) {
      logikaTanahAuto();
    }
  }

  if (now - tKirimSensor >= INTERVAL_KIRIM_SENSOR) {
    tKirimSensor = now;
    kirimSensor();
  }

  if (now - tKirimHarian >= INTERVAL_KIRIM_HARIAN) {
    tKirimHarian = now;
    kirimStatusHarian();
  }

  if (now - tLcd >= INTERVAL_LCD) {
    tLcd = now;
    tampilLcd();
  }

  if (now - tSimpan >= INTERVAL_SIMPan) {
    tSimpan = now;

    if (totalLightDuration != totalTerakhirDisimpan ||
        jumlahSiramHariIni != siramTerakhirDisimpan) {
      simpanData();
    }
  }
}
