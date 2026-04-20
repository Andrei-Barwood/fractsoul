(function () {
  const API_KEY_STORAGE = "fractsoul_energy_api_key";
  const API_KEY_HEADER = "X-API-Key";

  const state = {
    overview: null,
    selectedSiteID: "",
    operations: null,
    shadowPilot: null,
    reviews: [],
  };

  const refs = {
    apiKeyInput: document.getElementById("apiKeyInput"),
    apiKeyStatus: document.getElementById("apiKeyStatus"),
    saveApiKeyButton: document.getElementById("saveApiKeyButton"),
    refreshButton: document.getElementById("refreshButton"),
    campusMeta: document.getElementById("campusMeta"),
    siteCards: document.getElementById("siteCards"),
    detailTitle: document.getElementById("detailTitle"),
    detailMeta: document.getElementById("detailMeta"),
    kpiCurrentLoad: document.getElementById("kpiCurrentLoad"),
    kpiAllowedLoad: document.getElementById("kpiAllowedLoad"),
    kpiMargin: document.getElementById("kpiMargin"),
    kpiTariff: document.getElementById("kpiTariff"),
    kpiSacrificable: document.getElementById("kpiSacrificable"),
    kpiSafetyBlocked: document.getElementById("kpiSafetyBlocked"),
    riskChart: document.getElementById("riskChart"),
    riskProjectionList: document.getElementById("riskProjectionList"),
    constraintsList: document.getElementById("constraintsList"),
    recommendationsList: document.getElementById("recommendationsList"),
    blockedList: document.getElementById("blockedList"),
    reviewsList: document.getElementById("reviewsList"),
    shadowPilotSummary: document.getElementById("shadowPilotSummary"),
    explanationsList: document.getElementById("explanationsList"),
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
      let detail = `http_${response.status}`;
      try {
        const payload = await response.json();
        detail = payload.message || payload.code || detail;
      } catch (_) {
        // ignore parse errors and keep fallback status.
      }
      throw new Error(detail);
    }
    return response.json();
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

  function fmtKW(value) {
    return `${fmtNumber(value, 1)} kW`;
  }

  function fmtTime(value) {
    if (!value) {
      return "--";
    }
    return new Date(value).toLocaleString("es-CL", {
      hour12: false,
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
    });
  }

  function fmtShortTime(value) {
    if (!value) {
      return "--";
    }
    return new Date(value).toLocaleTimeString("es-CL", {
      hour12: false,
      hour: "2-digit",
      minute: "2-digit",
    });
  }

  function fmtCurrency(value) {
    if (!value) {
      return "--";
    }
    return `US$ ${fmtNumber(value, 0)}/MWh`;
  }

  function riskPillClass(level) {
    switch ((level || "").toLowerCase()) {
      case "critical":
        return "pill-critical";
      case "high":
        return "pill-high";
      case "moderate":
        return "pill-moderate";
      default:
        return "pill-low";
    }
  }

  function renderEmpty(target, message) {
    target.innerHTML = `<div class="empty-state">${message}</div>`;
  }

  function selectedOverviewSite() {
    const sites = (state.overview && state.overview.overview && state.overview.overview.sites) || [];
    return sites.find((item) => item.site_id === state.selectedSiteID) || null;
  }

  function renderSiteCards() {
    const overview = state.overview && state.overview.overview;
    const sites = (overview && overview.sites) || [];
    refs.campusMeta.textContent = overview
      ? `${overview.site_count} sitio(s) evaluados · cálculo ${fmtTime(overview.calculated_at)}`
      : "Sin datos.";

    if (!sites.length) {
      renderEmpty(refs.siteCards, "No hay sitios visibles para el principal actual.");
      return;
    }

    refs.siteCards.innerHTML = sites
      .map((site) => {
        const expensivePill = site.current_tariff_expensive
          ? `<span class="pill pill-expensive">${site.current_tariff_code || "tarifa cara"}</span>`
          : `<span class="pill pill-normal">${site.current_tariff_code || "sin tarifa"}</span>`;
        return `
          <button type="button" class="site-card ${site.site_id === state.selectedSiteID ? "active" : ""}" data-site-id="${site.site_id}">
            <div class="site-card-header">
              <div>
                <p class="section-kicker">${site.site_id}</p>
                <h3>${site.campus_name || site.site_id}</h3>
              </div>
              ${expensivePill}
            </div>
            <div class="site-card-stats">
              <div class="site-stat">
                <span>Consumo</span>
                <strong>${fmtKW(site.current_load_kw)}</strong>
              </div>
              <div class="site-stat">
                <span>Permitido</span>
                <strong>${fmtKW(site.allowed_load_kw)}</strong>
              </div>
              <div class="site-stat">
                <span>Margen</span>
                <strong>${fmtKW(site.margin_remaining_kw)}</strong>
              </div>
            </div>
            <div class="tag-row">
              <span class="pill pill-normal">${site.sacrificable_rack_count} sacrificables</span>
              <span class="pill pill-normal">${site.active_constraint_count} restricciones</span>
              <span class="pill ${riskPillClass(site.risk_projection?.[0]?.risk_level)}">${site.risk_projection?.[0]?.risk_level || "low"}</span>
            </div>
          </button>
        `;
      })
      .join("");

    refs.siteCards.querySelectorAll("[data-site-id]").forEach((button) => {
      button.addEventListener("click", async () => {
        state.selectedSiteID = button.getAttribute("data-site-id") || "";
        renderSiteCards();
        await loadSelectedSite();
      });
    });
  }

  function renderDetail() {
    const overviewSite = selectedOverviewSite();
    const operations = state.operations && state.operations.view;
    const shadow = state.shadowPilot && state.shadowPilot.result;

    if (!overviewSite || !operations) {
      refs.detailTitle.textContent = "Selecciona un sitio";
      refs.detailMeta.textContent = "Esperando datos.";
      refs.kpiCurrentLoad.textContent = "--";
      refs.kpiAllowedLoad.textContent = "--";
      refs.kpiMargin.textContent = "--";
      refs.kpiTariff.textContent = "--";
      refs.kpiSacrificable.textContent = "--";
      refs.kpiSafetyBlocked.textContent = "--";
      renderEmpty(refs.riskProjectionList, "Sin proyección disponible.");
      renderEmpty(refs.constraintsList, "Sin restricciones activas.");
      renderEmpty(refs.recommendationsList, "Sin recomendaciones pendientes.");
      renderEmpty(refs.blockedList, "Sin acciones bloqueadas.");
      renderEmpty(refs.reviewsList, "Sin reviews registrados.");
      renderEmpty(refs.shadowPilotSummary, "Sin resultados de piloto sombra.");
      renderEmpty(refs.explanationsList, "Sin explicaciones disponibles.");
      refs.riskChart.innerHTML = "";
      return;
    }

    refs.detailTitle.textContent = `${overviewSite.campus_name || overviewSite.site_id}`;
    refs.detailMeta.textContent = `${overviewSite.site_id} · cálculo ${fmtTime(overviewSite.calculated_at)}`;
    refs.kpiCurrentLoad.textContent = fmtKW(overviewSite.current_load_kw);
    refs.kpiAllowedLoad.textContent = fmtKW(overviewSite.allowed_load_kw);
    refs.kpiMargin.textContent = fmtKW(overviewSite.margin_remaining_kw);
    refs.kpiTariff.textContent = overviewSite.current_tariff_code
      ? `${overviewSite.current_tariff_code} · ${fmtCurrency(overviewSite.current_tariff_price_usd_per_mwh)}`
      : "--";
    refs.kpiSacrificable.textContent = String(overviewSite.sacrificable_rack_count || 0);
    refs.kpiSafetyBlocked.textContent = String(overviewSite.safety_blocked_rack_count || 0);

    renderRiskProjection(overviewSite.risk_projection || []);
    renderList(
      refs.constraintsList,
      operations.active_constraints || [],
      (item) => `
        <div class="list-item">
          <strong>${item.summary}</strong>
          <p>${item.explanation}</p>
          <div class="tag-row">
            <span class="pill ${riskPillClass(item.severity)}">${item.severity}</span>
            <span class="pill pill-normal">${item.scope}:${item.scope_id}</span>
            <span class="pill pill-normal">${item.code}</span>
          </div>
        </div>
      `,
      "Sin restricciones activas."
    );

    renderList(
      refs.recommendationsList,
      operations.pending_recommendations || [],
      (item) => `
        <div class="list-item">
          <strong>${item.action} ${item.rack_id || "site-wide"}</strong>
          <p>${item.explanation}</p>
          <div class="tag-row">
            <span class="pill pill-normal">${item.recommendation_id}</span>
            <span class="pill pill-normal">${item.criticality_class}</span>
            <span class="pill pill-normal">${fmtKW(item.recommended_delta_kw)}</span>
          </div>
        </div>
      `,
      "Sin recomendaciones pendientes."
    );

    renderList(
      refs.blockedList,
      operations.blocked_actions || [],
      (item) => `
        <div class="list-item">
          <strong>${item.attempted_action} ${item.rack_id || "site-wide"}</strong>
          <p>${item.explanation}</p>
          <div class="tag-row">
            <span class="pill pill-normal">${item.code}</span>
            <span class="pill pill-normal">${item.criticality_class}</span>
          </div>
        </div>
      `,
      "Sin acciones bloqueadas."
    );

    renderList(
      refs.reviewsList,
      state.reviews || [],
      (item) => `
        <div class="list-item">
          <strong>${item.recommendation_id} · ${item.status}</strong>
          <p>${(item.summary || []).join(" · ")}</p>
          <div class="event-row">
            ${(item.events || [])
              .map((event) => `<small>${fmtShortTime(event.created_at)} · ${event.actor_id} · ${event.event_type}</small>`)
              .join("")}
          </div>
        </div>
      `,
      "Sin reviews registrados."
    );

    renderShadowPilot(shadow);
    renderList(
      refs.explanationsList,
      operations.explanations || [],
      (item) => `
        <div class="list-item">
          <strong>${item.title}</strong>
          <p>${item.explanation}</p>
          <div class="tag-row">
            <span class="pill ${riskPillClass(item.severity)}">${item.severity}</span>
            <span class="pill pill-normal">${item.scope}:${item.scope_id}</span>
          </div>
        </div>
      `,
      "Sin explicaciones disponibles."
    );
  }

  function renderList(target, items, renderItem, emptyMessage) {
    if (!items.length) {
      renderEmpty(target, emptyMessage);
      return;
    }
    target.innerHTML = items.map(renderItem).join("");
  }

  function renderRiskProjection(points) {
    if (!points.length) {
      refs.riskChart.innerHTML = "";
      renderEmpty(refs.riskProjectionList, "Sin proyección disponible.");
      return;
    }

    const width = 720;
    const height = 220;
    const padding = 24;
    const values = points.flatMap((point) => [point.projected_load_kw, point.projected_safe_capacity_kw]);
    const minValue = Math.min(...values, 0);
    const maxValue = Math.max(...values, 1);
    const span = Math.max(maxValue - minValue, 1);
    const toX = (index) => padding + (index * (width - padding * 2)) / Math.max(points.length - 1, 1);
    const toY = (value) => height - padding - ((value - minValue) / span) * (height - padding * 2);
    const pathFor = (key) =>
      points
        .map((point, index) => `${index === 0 ? "M" : "L"}${toX(index).toFixed(1)},${toY(point[key]).toFixed(1)}`)
        .join(" ");

    const loadPath = pathFor("projected_load_kw");
    const safePath = pathFor("projected_safe_capacity_kw");

    refs.riskChart.innerHTML = `
      <defs>
        <linearGradient id="riskArea" x1="0%" y1="0%" x2="0%" y2="100%">
          <stop offset="0%" stop-color="rgba(86, 228, 198, 0.25)" />
          <stop offset="100%" stop-color="rgba(86, 228, 198, 0.02)" />
        </linearGradient>
      </defs>
      <path d="${safePath}" fill="none" stroke="#6ae09d" stroke-width="3" stroke-linejoin="round"></path>
      <path d="${loadPath}" fill="none" stroke="#ffb252" stroke-width="3" stroke-linejoin="round"></path>
      ${points
        .map((point, index) => {
          const x = toX(index).toFixed(1);
          const y = toY(point.projected_load_kw).toFixed(1);
          const color =
            point.risk_level === "critical"
              ? "#ff6f61"
              : point.risk_level === "high"
                ? "#ff9353"
                : point.risk_level === "moderate"
                  ? "#ffb252"
                  : "#56e4c6";
          return `<circle cx="${x}" cy="${y}" r="5" fill="${color}"></circle>`;
        })
        .join("")}
      ${points
        .map((point, index) => {
          const x = toX(index).toFixed(1);
          return `<text x="${x}" y="${height - 6}" text-anchor="middle" fill="#99bcc6" font-size="11">${fmtShortTime(point.at)}</text>`;
        })
        .join("")}
    `;

    refs.riskProjectionList.innerHTML = points
      .map(
        (point) => `
          <div class="projection-card">
            <strong>${fmtShortTime(point.at)}</strong>
            <p>${fmtKW(point.projected_load_kw)} / ${fmtKW(point.projected_safe_capacity_kw)}</p>
            <div class="tag-row">
              <span class="pill ${riskPillClass(point.risk_level)}">${point.risk_level}</span>
              <span class="pill pill-normal">score ${fmtNumber(point.risk_score, 0)}</span>
            </div>
            <small>${(point.reasons || []).join(", ") || "sin factores críticos"}</small>
          </div>
        `
      )
      .join("");
  }

  function renderShadowPilot(shadow) {
    if (!shadow) {
      renderEmpty(refs.shadowPilotSummary, "Sin resultados de piloto sombra.");
      return;
    }

    const gaps = (shadow.missing_data || [])
      .map((item) => `<small>${item.code}: ${item.count}</small>`)
      .join("");

    refs.shadowPilotSummary.innerHTML = `
      <div class="list-item">
        <strong>${shadow.site_id} · ${shadow.day.slice(0, 10)}</strong>
        <p>${(shadow.summary || []).join(" ")}</p>
        <div class="tag-row">
          <span class="pill pill-normal">${shadow.recommendations_evaluated} evaluadas</span>
          <span class="pill pill-normal">${shadow.decisions_correct} correctas</span>
          <span class="pill pill-normal">${shadow.decisions_blocked} bloqueadas</span>
          <span class="pill pill-normal">${shadow.decisions_would_escalate} escalan</span>
        </div>
        <div class="event-row">${gaps || "<small>sin brechas de datos relevantes</small>"}</div>
      </div>
    `;
  }

  async function loadOverview() {
    state.overview = await fetchJSON("/v1/energy/overview");
    const sites = (state.overview.overview && state.overview.overview.sites) || [];
    if (!state.selectedSiteID || !sites.some((item) => item.site_id === state.selectedSiteID)) {
      state.selectedSiteID = sites[0] ? sites[0].site_id : "";
    }
    renderSiteCards();
  }

  async function loadSelectedSite() {
    if (!state.selectedSiteID) {
      renderDetail();
      return;
    }

    const day = new Date().toISOString().slice(0, 10);
    const [operations, reviews, shadowPilot] = await Promise.all([
      fetchJSON(`/v1/energy/sites/${state.selectedSiteID}/operations`),
      fetchJSON(`/v1/energy/sites/${state.selectedSiteID}/recommendations/reviews`).catch(() => ({ items: [] })),
      fetchJSON(`/v1/energy/sites/${state.selectedSiteID}/pilot/shadow`, { day }).catch(() => null),
    ]);

    state.operations = operations;
    state.reviews = reviews.items || [];
    state.shadowPilot = shadowPilot;
    renderDetail();
  }

  async function refresh() {
    try {
      refs.apiKeyStatus.textContent = currentApiKey()
        ? "API key cargada en localStorage."
        : "Auth deshabilitada o sin key local.";
      await loadOverview();
      await loadSelectedSite();
      refs.lastUpdated.textContent = `Actualizado ${fmtTime(new Date().toISOString())}`;
    } catch (error) {
      if (error.message === "auth") {
        refs.apiKeyStatus.textContent = "La API respondió 401. Revisa la key.";
      } else {
        refs.apiKeyStatus.textContent = `Error: ${error.message}`;
      }
      console.error(error);
    }
  }

  refs.saveApiKeyButton.addEventListener("click", () => {
    const value = refs.apiKeyInput.value.trim();
    if (!value) {
      window.localStorage.removeItem(API_KEY_STORAGE);
      refs.apiKeyStatus.textContent = "API key eliminada del navegador local.";
      return;
    }
    window.localStorage.setItem(API_KEY_STORAGE, value);
    refs.apiKeyStatus.textContent = "API key guardada localmente.";
  });

  refs.refreshButton.addEventListener("click", refresh);

  refs.apiKeyInput.value = currentApiKey();
  refresh();
})();
