(function () {
  const DEFAULT_LIMIT = 200;
  const API_KEY_STORAGE = "fractsoul_api_key";
  const API_KEY_HEADER = "X-API-Key";

  const state = {
    site: "",
    rack: "",
    model: "",
    miner: "",
    readings: [],
    summary: null,
  };

  const refs = {
    apiKeyInput: document.getElementById("apiKeyInput"),
    apiKeyStatus: document.getElementById("apiKeyStatus"),
    saveApiKeyButton: document.getElementById("saveApiKeyButton"),
    refreshButton: document.getElementById("refreshButton"),
    siteFilter: document.getElementById("siteFilter"),
    rackFilter: document.getElementById("rackFilter"),
    modelFilter: document.getElementById("modelFilter"),
    minerFilter: document.getElementById("minerFilter"),
    kpiHashrate: document.getElementById("kpiHashrate"),
    kpiPower: document.getElementById("kpiPower"),
    kpiTemp: document.getElementById("kpiTemp"),
    kpiCriticalRatio: document.getElementById("kpiCriticalRatio"),
    kpiSampleCount: document.getElementById("kpiSampleCount"),
    minerTitle: document.getElementById("minerTitle"),
    minerModel: document.getElementById("minerModel"),
    minerStatus: document.getElementById("minerStatus"),
    minerHashrate: document.getElementById("minerHashrate"),
    minerPower: document.getElementById("minerPower"),
    minerTemp: document.getElementById("minerTemp"),
    minerFans: document.getElementById("minerFans"),
    chart: document.getElementById("timeseriesChart"),
    tableBody: document.getElementById("readingsTableBody"),
    lastUpdated: document.getElementById("lastUpdated"),
  };

  function currentApiKey() {
    return window.localStorage.getItem(API_KEY_STORAGE) || "";
  }

  function authHeaders() {
    const apiKey = currentApiKey();
    if (!apiKey) {
      return {};
    }
    return { [API_KEY_HEADER]: apiKey };
  }

  function apiURL(path, params) {
    const url = new URL(path, window.location.origin);
    if (params) {
      Object.entries(params).forEach(([key, value]) => {
        if (value === undefined || value === null || value === "") {
          return;
        }
        url.searchParams.set(key, value);
      });
    }
    return url.toString();
  }

  async function fetchJSON(path, params) {
    const response = await fetch(apiURL(path, params), { headers: authHeaders() });
    if (response.status === 401) {
      throw new Error("auth");
    }
    if (!response.ok) {
      throw new Error(`http_${response.status}`);
    }
    return response.json();
  }

  function populateSelect(select, values, currentValue, placeholder) {
    const sorted = [...new Set(values)].sort((a, b) => a.localeCompare(b));
    const options = [`<option value="">${placeholder}</option>`]
      .concat(sorted.map((value) => `<option value="${value}">${value}</option>`))
      .join("");
    select.innerHTML = options;
    if (currentValue && sorted.includes(currentValue)) {
      select.value = currentValue;
    }
  }

  function updateFilterOptions() {
    const readings = state.readings;
    const sites = readings.map((item) => item.site_id);
    const racks = readings
      .filter((item) => !state.site || item.site_id === state.site)
      .map((item) => item.rack_id);
    const models = readings
      .filter((item) => {
        if (state.site && item.site_id !== state.site) {
          return false;
        }
        if (state.rack && item.rack_id !== state.rack) {
          return false;
        }
        return true;
      })
      .map((item) => item.miner_model || "UNKNOWN");
    const miners = readings
      .filter((item) => {
        if (state.site && item.site_id !== state.site) {
          return false;
        }
        if (state.rack && item.rack_id !== state.rack) {
          return false;
        }
        if (state.model && (item.miner_model || "UNKNOWN") !== state.model) {
          return false;
        }
        return true;
      })
      .map((item) => item.miner_id);

    populateSelect(refs.siteFilter, sites, state.site, "Todos");
    populateSelect(refs.rackFilter, racks, state.rack, "Todos");
    populateSelect(refs.modelFilter, models, state.model, "Todos");
    populateSelect(refs.minerFilter, miners, state.miner, "Selecciona");
  }

  function fmtNumber(value, decimals = 1) {
    if (value === undefined || value === null || Number.isNaN(Number(value))) {
      return "--";
    }
    return Number(value).toLocaleString("es-CL", {
      minimumFractionDigits: decimals,
      maximumFractionDigits: decimals,
    });
  }

  function renderKPIs() {
    const summary = state.summary || {};
    refs.kpiHashrate.textContent = fmtNumber(summary.avg_hashrate_ths, 1);
    refs.kpiPower.textContent = fmtNumber(summary.avg_power_watts, 0);
    refs.kpiTemp.textContent = fmtNumber(summary.avg_temp_celsius, 1);

    const criticalCount = state.readings.filter(
      (item) => item.status === "critical" || item.status === "offline",
    ).length;
    const total = state.readings.length || 1;
    const ratio = (criticalCount * 100) / total;
    refs.kpiCriticalRatio.textContent = `${fmtNumber(ratio, 1)}%`;
    refs.kpiSampleCount.textContent = `${summary.samples || 0} muestras`;
  }

  function statusBadge(status) {
    const safe = (status || "unknown").toLowerCase();
    return `<span class="status-badge status-${safe}">${safe}</span>`;
  }

  function renderTable() {
    refs.tableBody.innerHTML = state.readings
      .slice(0, 80)
      .map((item) => {
        const time = new Date(item.timestamp).toLocaleTimeString("es-CL", {
          hour: "2-digit",
          minute: "2-digit",
          second: "2-digit",
        });
        return `
          <tr>
            <td>${time}</td>
            <td>${item.site_id}</td>
            <td>${item.rack_id}</td>
            <td>${item.miner_id}</td>
            <td>${item.miner_model || "UNKNOWN"}</td>
            <td>${statusBadge(item.status)}</td>
            <td>${fmtNumber(item.hashrate_ths, 1)}</td>
            <td>${fmtNumber(item.power_watts, 0)}</td>
            <td>${fmtNumber(item.temp_celsius, 1)}</td>
          </tr>
        `;
      })
      .join("");
  }

  function renderDetailFromLatest(latest) {
    if (!latest) {
      refs.minerTitle.textContent = "Selecciona una máquina para ver detalle.";
      refs.minerModel.textContent = "--";
      refs.minerStatus.textContent = "--";
      refs.minerHashrate.textContent = "--";
      refs.minerPower.textContent = "--";
      refs.minerTemp.textContent = "--";
      refs.minerFans.textContent = "--";
      refs.chart.innerHTML = "";
      return;
    }

    refs.minerTitle.textContent = `${latest.miner_id} · ${latest.site_id} · ${latest.rack_id}`;
    refs.minerModel.textContent = latest.miner_model || "UNKNOWN";
    refs.minerStatus.textContent = latest.status || "--";
    refs.minerHashrate.textContent = `${fmtNumber(latest.hashrate_ths, 1)} TH/s`;
    refs.minerPower.textContent = `${fmtNumber(latest.power_watts, 0)} W`;
    refs.minerTemp.textContent = `${fmtNumber(latest.temp_celsius, 1)} °C`;
    refs.minerFans.textContent = `${fmtNumber(latest.fan_rpm, 0)} rpm`;
  }

  function buildPath(points, key, width, height, min, max) {
    if (!points.length) {
      return "";
    }
    const span = Math.max(max - min, 1);
    return points
      .map((point, index) => {
        const x = (index / Math.max(points.length - 1, 1)) * (width - 30) + 15;
        const y = height - (((point[key] - min) / span) * (height - 30) + 15);
        return `${index === 0 ? "M" : "L"}${x.toFixed(1)},${y.toFixed(1)}`;
      })
      .join(" ");
  }

  function renderTimeseries(points) {
    refs.chart.innerHTML = "";
    if (!points.length) {
      return;
    }

    const width = 640;
    const height = 220;
    const temps = points.map((point) => Number(point.avg_temp_celsius || 0));
    const hashes = points.map((point) => Number(point.avg_hashrate_ths || 0));
    const tempPath = buildPath(points, "avg_temp_celsius", width, height, Math.min(...temps), Math.max(...temps));
    const hashPath = buildPath(points, "avg_hashrate_ths", width, height, Math.min(...hashes), Math.max(...hashes));

    refs.chart.innerHTML = `
      <line x1="15" y1="205" x2="625" y2="205" stroke="rgba(159,196,215,0.35)" />
      <path d="${hashPath}" stroke="#37d0bf" stroke-width="3" fill="none" />
      <path d="${tempPath}" stroke="#ff9f43" stroke-width="3" fill="none" />
      <text x="18" y="24" fill="#7de9db" font-size="12">Hashrate</text>
      <text x="100" y="24" fill="#ffc17f" font-size="12">Temp</text>
    `;
  }

  async function refreshDashboard() {
    try {
      const filters = {
        site_id: state.site,
        rack_id: state.rack,
        model: state.model,
        limit: DEFAULT_LIMIT,
      };
      const [readingsResp, summaryResp] = await Promise.all([
        fetchJSON("/v1/telemetry/readings", filters),
        fetchJSON("/v1/telemetry/summary", {
          site_id: state.site,
          rack_id: state.rack,
          model: state.model,
          window_minutes: 60,
        }),
      ]);

      state.readings = readingsResp.items || [];
      state.summary = summaryResp.summary || {};

      updateFilterOptions();
      renderKPIs();
      renderTable();

      if (!state.miner && state.readings.length > 0) {
        state.miner = state.readings[0].miner_id;
        refs.minerFilter.value = state.miner;
      }

      await refreshMinerDetail();

      refs.lastUpdated.textContent = `Actualizado: ${new Date().toLocaleString("es-CL")} · ${state.readings.length} lecturas`;
      refs.apiKeyStatus.textContent = currentApiKey()
        ? "API key local cargada."
        : "Auth deshabilitada o sin key local.";
    } catch (error) {
      if (String(error.message || "").includes("auth")) {
        refs.apiKeyStatus.textContent = "API key faltante o inválida.";
      }
      refs.lastUpdated.textContent = `Error actualizando dashboard: ${error.message}`;
    }
  }

  async function refreshMinerDetail() {
    if (!state.miner) {
      renderDetailFromLatest(null);
      renderTimeseries([]);
      return;
    }

    const latestResp = await fetchJSON("/v1/telemetry/readings", { miner_id: state.miner, limit: 1 });
    const latest = (latestResp.items || [])[0];
    renderDetailFromLatest(latest);

    const to = new Date();
    const from = new Date(to.getTime() - 2 * 60 * 60 * 1000);
    const seriesResp = await fetchJSON(`/v1/telemetry/miners/${state.miner}/timeseries`, {
      resolution: "minute",
      from: from.toISOString(),
      to: to.toISOString(),
      limit: 180,
    });
    renderTimeseries(seriesResp.items || []);
  }

  function bindEvents() {
    refs.saveApiKeyButton.addEventListener("click", () => {
      const value = refs.apiKeyInput.value.trim();
      if (value) {
        window.localStorage.setItem(API_KEY_STORAGE, value);
        refs.apiKeyStatus.textContent = "API key guardada localmente.";
      } else {
        window.localStorage.removeItem(API_KEY_STORAGE);
        refs.apiKeyStatus.textContent = "API key eliminada.";
      }
      refreshDashboard();
    });

    refs.refreshButton.addEventListener("click", refreshDashboard);

    refs.siteFilter.addEventListener("change", () => {
      state.site = refs.siteFilter.value;
      if (state.site && state.rack && !state.rack.startsWith(`rack-${state.site.split("-")[1]}-${state.site.split("-")[2]}`)) {
        state.rack = "";
      }
      refreshDashboard();
    });
    refs.rackFilter.addEventListener("change", () => {
      state.rack = refs.rackFilter.value;
      refreshDashboard();
    });
    refs.modelFilter.addEventListener("change", () => {
      state.model = refs.modelFilter.value;
      refreshDashboard();
    });
    refs.minerFilter.addEventListener("change", () => {
      state.miner = refs.minerFilter.value;
      refreshMinerDetail().catch((error) => {
        refs.lastUpdated.textContent = `Error cargando detalle: ${error.message}`;
      });
    });
  }

  function init() {
    refs.apiKeyInput.value = currentApiKey();
    bindEvents();
    refreshDashboard();
    window.setInterval(refreshDashboard, 30000);
  }

  init();
})();
