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

      const chart = new Chart(canvas, {
        type: 'line',
        data: {
          labels: [],
          datasets: keys.map((key) => ({
            label: `${SENSORS[key].label} (${SENSORS[key].unit})`,
            data: [],
            borderColor: SENSORS[key].color,
            backgroundColor: `${SENSORS[key].color}22`,
            borderWidth: 2,
            pointRadius: 0,
            tension: 0.35,
            fill: single,
          })),
        },
        options: {
          responsive: true,
          maintainAspectRatio: false,
          animation: false,
          interaction: { mode: 'index', intersect: false },
          plugins: { legend: { display: !single, position: 'bottom' } },
          scales: { x: { grid: { display: false }, ticks: { maxTicksLimit: 6 } } },
        },
      });

      charts.push({ chart, keys });
    });
  }

  async function loadHistory() {
    if (charts.length === 0) return;

    try {
      const res = await fetch(`/api/sensors/history?limit=${maxPoints}`, { credentials: 'same-origin' });
      const { data } = await res.json();

      charts.forEach(({ chart, keys }) => {
        chart.data.labels = data.map((r) => timeLabel(r.created_at));
        chart.data.datasets.forEach((ds, i) => (ds.data = data.map((r) => r[keys[i]])));
        chart.update();
      });
      updateStats();
    } catch (err) {
      toast('Gagal memuat riwayat grafik', 'danger');
    }
  }

  function pushReading(reading) {
    charts.forEach(({ chart, keys }) => {
      chart.data.labels.push(timeLabel(reading.created_at));
      chart.data.datasets.forEach((ds, i) => ds.data.push(reading[keys[i]]));

      while (chart.data.labels.length > maxPoints) {
        chart.data.labels.shift();
        chart.data.datasets.forEach((ds) => ds.data.shift());
      }
      chart.update();
    });
    updateStats();
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
