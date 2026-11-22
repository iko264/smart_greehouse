(() => {
  'use strict';

  // ---------------------------------------------------------------------------
  // Konfigurasi sensor (samakan dengan support.Sensors di app/support/helper.go)
  // ---------------------------------------------------------------------------
  const SENSORS = {
    temperature: { label: 'Suhu', unit: '°C', decimals: 1, color: '#dc3545' },
    humidity: { label: 'Kelembapan Udara', unit: '%', decimals: 1, color: '#0d6efd' },
    soil: { label: 'Kelembapan Tanah', unit: '%', decimals: 0, color: '#198754' },
    light: { label: 'Cahaya', unit: 'lux', decimals: 0, color: '#d39e00' },
  };

  const OFFLINE_AFTER_MS = 30000; // tidak ada data > 30 detik = perangkat dianggap offline

  const $ = (selector, root = document) => root.querySelector(selector);
  const $$ = (selector, root = document) => [...root.querySelectorAll(selector)];
  const csrfToken = $('meta[name="csrf-token"]')?.content || '';

  const fmt = (code, value) => Number(value).toFixed(SENSORS[code]?.decimals ?? 1);
  const timeLabel = (iso) => new Date(iso).toLocaleTimeString('en-GB');

  // ---------------------------------------------------------------------------
  // Toast & request ke server
  // ---------------------------------------------------------------------------
  function toast(message, type = 'success') {
    const box = $('#toast-container');
    if (!box) return;

    const el = document.createElement('div');
    el.className = `toast align-items-center text-bg-${type} border-0`;
    el.setAttribute('role', 'alert');
    el.innerHTML =
      '<div class="d-flex"><div class="toast-body"></div>' +
      '<button type="button" class="btn-close btn-close-white me-2 m-auto" data-bs-dismiss="toast"></button></div>';
    el.querySelector('.toast-body').textContent = message;

    box.appendChild(el);
    el.addEventListener('hidden.bs.toast', () => el.remove());
    new bootstrap.Toast(el, { delay: 3500 }).show();
  }

  async function postForm(url, data) {
    const res = await fetch(url, {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'X-Requested-With': 'XMLHttpRequest', Accept: 'application/json' },
      body: new URLSearchParams({ ...data, _token: csrfToken }),
    });
    const json = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(json.message || `Gagal (${res.status})`);
    return json;
  }

  // ---------------------------------------------------------------------------
  // Status perangkat (online / offline berdasarkan data terakhir)
  // ---------------------------------------------------------------------------
  let lastSeen = null;

  function setLastSeen(iso) {
    lastSeen = iso ? new Date(iso) : null;
    const label = $('#last-update');
    if (label && lastSeen) label.textContent = lastSeen.toLocaleString('id-ID');
    renderDeviceStatus();
  }

  function renderDeviceStatus() {
    const badge = $('#device-status');
    if (!badge) return;

    const online = lastSeen && Date.now() - lastSeen.getTime() < OFFLINE_AFTER_MS;
    badge.className = `badge ${online ? 'text-bg-light' : 'text-bg-warning'}`;
    $('.label', badge).textContent = online ? 'Perangkat online' : 'Tidak ada data';
  }

  function renderMqttStatus(connected) {
    const badge = $('#mqtt-status');
    if (!badge) return;

    badge.className = `badge ${connected ? 'text-bg-light' : 'text-bg-danger'}`;
    $('.label', badge).textContent = connected ? 'Terhubung' : 'Terputus';
  }

  // ---------------------------------------------------------------------------
  // Grafik (Chart.js)
  // ---------------------------------------------------------------------------
  const charts = []; // { chart, keys }
  let maxPoints = 30;

  function createCharts() {
    if (typeof Chart === 'undefined') return;

    $$('canvas[data-keys]').forEach((canvas) => {
      const keys = canvas.dataset.keys.split(',');
      const single = keys.length === 1;

      const datasets = keys.map((key) => ({
        label: `${SENSORS[key].label} (${SENSORS[key].unit})`,
        data: [],
        borderColor: SENSORS[key].color,
        backgroundColor: `${SENSORS[key].color}22`,
        borderWidth: 2,
        pointRadius: 0,
        tension: 0.35,
        fill: single,
      }));

      if (single) {
        // Titik anomali (|z| >= ambang) diberi bulatan merah
        datasets[0].pointBackgroundColor = '#dc3545';
        datasets[0].pointRadius = (ctx) => (ctx.chart.$anomaly?.[ctx.dataIndex] ? 5 : 0);

        // Garis putus-putus = moving average
        datasets.push({
          label: 'Moving average',
          data: [],
          borderColor: '#6c757d',
          borderDash: [6, 4],
          borderWidth: 2,
          pointRadius: 0,
          tension: 0.35,
          fill: false,
        });
      }

      const chart = new Chart(canvas, {
        type: 'line',
        data: { labels: [], datasets },
        options: {
          responsive: true,
          maintainAspectRatio: false,
          animation: false,
          interaction: { mode: 'index', intersect: false },
          plugins: { legend: { display: true, position: 'bottom' } },
          scales: { x: { grid: { display: false }, ticks: { maxTicksLimit: 6 } } },
        },
      });
      chart.$anomaly = []; // status anomali per titik (sejajar dengan labels)

      charts.push({ chart, keys });
    });
  }

  // Mengisi grafik dari daftar data (hasil /api/sensors/history)
  function fillChart({ chart, keys }, rows) {
    chart.data.labels = rows.map((r) => timeLabel(r.created_at));
    keys.forEach((key, i) => (chart.data.datasets[i].data = rows.map((r) => r[key])));

    if (keys.length === 1) {
      chart.data.datasets[1].data = rows.map((r) => r.ma[keys[0]]);
      chart.$anomaly = rows.map((r) => r.anomaly[keys[0]]);
    }
    chart.update();
  }

  async function loadHistory() {
    if (charts.length === 0) return;

    try {
      const res = await fetch(`/api/sensors/history?limit=${maxPoints}`, { credentials: 'same-origin' });
      const { data } = await res.json();

      charts.forEach((c) => fillChart(c, data));
      updateStats();
      if (data.length > 0) renderAnalysis(data[data.length - 1]);
    } catch (err) {
      toast('Gagal memuat riwayat grafik', 'danger');
    }
  }

  function pushReading(reading) {
    charts.forEach(({ chart, keys }) => {
      chart.data.labels.push(timeLabel(reading.created_at));
      keys.forEach((key, i) => chart.data.datasets[i].data.push(reading[key]));

      if (keys.length === 1) {
        chart.data.datasets[1].data.push(reading.ma[keys[0]]);
        chart.$anomaly.push(reading.anomaly[keys[0]]);
      }

      while (chart.data.labels.length > maxPoints) {
        chart.data.labels.shift();
        chart.data.datasets.forEach((ds) => ds.data.shift());
        chart.$anomaly.shift();
      }
      chart.update();
    });
    updateStats();
  }

  // Menampilkan "MA ... · z ... [Anomali]" di kartu sensor
  function renderAnalysis(r) {
    Object.keys(SENSORS).forEach((code) => {
      $$(`[data-analysis="${code}"]`).forEach((box) => {
        $('[data-k="ma"]', box).textContent = fmt(code, r.ma[code]);
        $('[data-k="z"]', box).textContent = Number(r.z[code]).toFixed(2);
        $('[data-k="flag"]', box).classList.toggle('d-none', !r.anomaly[code]);
      });
    });
  }

  // Terendah / rata-rata / tertinggi dari data yang sedang tampil di grafik (halaman Monitoring)
  function updateStats() {
    charts.forEach(({ chart, keys }) => {
      if (keys.length !== 1) return;

      const box = $(`[data-stats="${keys[0]}"]`);
      const values = chart.data.datasets[0].data;
      if (!box || values.length === 0) return;

      const sum = values.reduce((a, b) => a + b, 0);
      $('[data-k="min"]', box).textContent = fmt(keys[0], Math.min(...values));
      $('[data-k="avg"]', box).textContent = fmt(keys[0], sum / values.length);
      $('[data-k="max"]', box).textContent = fmt(keys[0], Math.max(...values));
    });
  }

  // ---------------------------------------------------------------------------
  // Update tampilan dari data real-time
  // ---------------------------------------------------------------------------
  function onSensor(reading) {
    Object.keys(SENSORS).forEach((code) => {
      $$(`[data-sensor-value="${code}"]`).forEach((el) => (el.textContent = fmt(code, reading[code])));
    });
    setLastSeen(reading.created_at);
    if (reading.ma) renderAnalysis(reading);
    pushReading(reading);
  }

  function onActuator(a) {
    $$(`.actuator-switch[data-code="${a.code}"]`).forEach((el) => {
      el.checked = a.state;
      el.disabled = a.mode === 'auto';
    });
    $$(`.actuator-state[data-code="${a.code}"]`).forEach((el) => {
      el.textContent = a.state ? 'ON' : 'OFF';
      el.className = `badge actuator-state ${a.state ? 'text-bg-success' : 'text-bg-secondary'}`;
    });
    $$(`.actuator-mode[data-code="${a.code}"]`).forEach((el) => {
      el.textContent = a.mode === 'auto' ? 'Otomatis' : 'Manual';
    });
    $$(`.mode-btn[data-code="${a.code}"]`).forEach((el) => {
      el.classList.toggle('active', el.dataset.mode === a.mode);
    });
  }

  // ---------------------------------------------------------------------------
  // Widget Manajemen Cahaya & Penyiraman Harian
  // ---------------------------------------------------------------------------
  const daily = { lightSeconds: 0, waterCount: 0, lightTarget: 16, maxWatering: 6 };

  function fmtJam(detik) {
    return (detik / 3600).toFixed(1);
  }

  function renderDaily() {
    const lightHours = fmtJam(daily.lightSeconds);
    const lightPct = Math.min(100, (daily.lightSeconds / (daily.lightTarget * 3600)) * 100);
    const waterPct = Math.min(100, (daily.waterCount / daily.maxWatering) * 100);

    $('#light-hours').textContent = lightHours;
    $('#light-target-label').textContent = daily.lightTarget;
    $('#light-progress').style.width = `${lightPct}%`;

    $('#water-count').textContent = daily.waterCount;
    $('#water-max-label').textContent = daily.maxWatering;
    $('#water-progress').style.width = `${waterPct}%`;

    $('#light-target-input').value = daily.lightTarget;
    $('#max-water-input').value = daily.maxWatering;
  }

  function applyDaily(d) {
    if (typeof d.light_seconds === 'number') daily.lightSeconds = d.light_seconds;
    if (typeof d.water_count === 'number') daily.waterCount = d.water_count;
    if (typeof d.light_target_hours === 'number') daily.lightTarget = d.light_target_hours;
    if (typeof d.max_watering_per_day === 'number') daily.maxWatering = d.max_watering_per_day;
    renderDaily();
  }

  async function loadDaily() {
    try {
      const [dailyRes, limitsRes] = await Promise.all([
        fetch('/api/greenhouse/daily', { credentials: 'same-origin' }),
        fetch('/api/greenhouse/limits', { credentials: 'same-origin' }),
      ]);
      const dailyJson = await dailyRes.json();
      const limitsJson = await limitsRes.json();
      applyDaily({ ...dailyJson.data, ...limitsJson.data });
    } catch (err) {
      /* biarkan nilai default tampil */
    }
  }

  document.getElementById('limits-form')?.addEventListener('submit', async (event) => {
    event.preventDefault();
    const form = event.target;
    const btn = $('#limits-save');
    const lightTarget = Number($('#light-target-input').value);
    const maxWater = Number($('#max-water-input').value);

    if (!(lightTarget >= 1 && lightTarget <= 24) || !(maxWater >= 1 && maxWater <= 50)) {
      toast('Target cahaya 1-24 jam, batas siram 1-50x', 'danger');
      return;
    }

    btn.disabled = true;
    try {
      const res = await postForm('/greenhouse/limits', {
        light_target_hours: lightTarget,
        max_watering_per_day: maxWater,
      });
      applyDaily(res.data);
      toast(res.message || 'Pengaturan tersimpan');
    } catch (err) {
      toast(err.message, 'danger');
    }
    btn.disabled = false;
  });

  // ---------------------------------------------------------------------------
  // WebSocket (otomatis menyambung ulang kalau putus)
  // ---------------------------------------------------------------------------
  let retry = 0;

  function connectSocket() {
    const scheme = location.protocol === 'https:' ? 'wss' : 'ws';
    const socket = new WebSocket(`${scheme}://${location.host}/ws`);

    socket.onopen = () => (retry = 0);

    socket.onmessage = (event) => {
      const msg = JSON.parse(event.data);
      if (msg.event === 'sensor') onSensor(msg.data);
      else if (msg.event === 'actuator') onActuator(msg.data);
      else if (msg.event === 'mqtt') renderMqttStatus(msg.data.connected);
      else if (msg.event === 'daily') applyDaily(msg.data);
    };

    socket.onclose = () => {
      const delay = Math.min(1000 * 2 ** retry++, 15000);
      setTimeout(connectSocket, delay);
    };
  }

  // ---------------------------------------------------------------------------
  // Aksi pengguna: switch aktuator & tombol mode
  // ---------------------------------------------------------------------------
  document.addEventListener('change', async (event) => {
    const sw = event.target.closest?.('.actuator-switch');
    if (!sw) return;

    const wantOn = sw.checked;
    sw.disabled = true;
    try {
      const res = await postForm(`/actuators/${sw.dataset.code}/command`, { state: wantOn ? 'on' : 'off' });
      onActuator(res.data);
      toast(`${res.data.name} ${wantOn ? 'dinyalakan' : 'dimatikan'}`);
    } catch (err) {
      sw.checked = !wantOn; // kembalikan switch ke posisi semula
      sw.disabled = false;
      toast(err.message, 'danger');
    }
  });

  document.addEventListener('click', async (event) => {
    const btn = event.target.closest?.('.mode-btn');
    if (!btn) return;

    try {
      const res = await postForm(`/actuators/${btn.dataset.code}/mode`, { mode: btn.dataset.mode });
      onActuator(res.data);
      toast(`${res.data.name}: mode ${res.data.mode === 'auto' ? 'otomatis' : 'manual'}`);
    } catch (err) {
      toast(err.message, 'danger');
    }
  });

  // ---------------------------------------------------------------------------
  // Mulai
  // ---------------------------------------------------------------------------
  document.addEventListener('DOMContentLoaded', async () => {
    // Halaman login tidak punya navbar, jadi tidak perlu data real-time.
    if (!$('#mqtt-status')) return;

    createCharts();

    $('#range-select')?.addEventListener('change', (e) => {
      maxPoints = Number(e.target.value);
      loadHistory();
    });
    if ($('#range-select')) maxPoints = Number($('#range-select').value);

    loadHistory();
    loadDaily();
    connectSocket();

    // Status perangkat: ambil waktu data terakhir, lalu cek ulang tiap 5 detik
    try {
      const res = await fetch('/api/sensors/latest', { credentials: 'same-origin' });
      const { data } = await res.json();
      if (data) setLastSeen(data.created_at);
    } catch (err) {
      /* abaikan */
    }
    renderDeviceStatus();
    setInterval(renderDeviceStatus, 5000);
  });
})();
