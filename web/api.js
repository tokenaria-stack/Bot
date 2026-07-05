/**
 * REST transport layer — fetch only, no Store/DOM/render.
 */
const API = {
  get LIVE_STATE_CANDLE_LIMIT() {
    return (typeof CONFIG !== 'undefined' && CONFIG.LIVE_STATE_CANDLE_LIMIT) || 3000;
  },
  get LIVE_POLL_CANDLE_LIMIT() {
    return (typeof CONFIG !== 'undefined' && CONFIG.LIVE_POLL_CANDLE_LIMIT) || 5;
  },
  LIVE_STATE_FETCH_TIMEOUT_MS: 10000,

  _liveStateAbort: null,
  _liveStateInflight: new Map(),

  // Универсальный конвертер PascalCase -> camelCase для ответов Go
  normalizeGoResponse(obj) {
    if (Array.isArray(obj)) {
      return obj.map(API.normalizeGoResponse);
    }
    if (obj !== null && typeof obj === 'object') {
      const normalized = {};
      for (const [key, value] of Object.entries(obj)) {
        const camelKey = key.charAt(0).toLowerCase() + key.slice(1);
        normalized[camelKey] = API.normalizeGoResponse(value);
      }
      return normalized;
    }
    return obj;
  },

  apiFetchUrl(path, params = {}) {
    const qs = new URLSearchParams();
    Object.entries(params).forEach(([key, value]) => {
      if (value != null && value !== '') qs.set(key, String(value));
    });
    qs.set('_t', String(Date.now()));
    return `${path}?${qs.toString()}`;
  },

  /**
   * Build /api/state query params. Caller supplies tf, lookback, limit, optional anchor.
   */
  apiQueryParams(config = {}) {
    const {
      tf,
      rsxLookback,
      limit,
      extra = {},
    } = config;

    const params = {
      tf,
      rsxLookback,
      limit,
      ...extra,
    };

    return params;
  },

  abortLiveStateFetch() {
    if (API._liveStateAbort) {
      API._liveStateAbort.abort();
      API._liveStateAbort = null;
    }
    API._liveStateInflight.clear();
  },

  liveStateRequestKey(tf, options = {}) {
    if (options.navigatorsOnly) return `nav:${tf}`;
    return `tf:${tf}`;
  },

  async fetchState(options = {}) {
    const {
      params = {},
      navigatorsOnly = false,
      signal = null,
      timeoutMs = API.LIVE_STATE_FETCH_TIMEOUT_MS,
    } = options;

    const queryParams = { ...params };
    if (navigatorsOnly) {
      queryParams.navigators = '1';
      queryParams.limit = 0;
    }

    const timeoutController = new AbortController();
    const timeoutId = setTimeout(() => {
      timeoutController.abort(new DOMException('fetchState timeout', 'TimeoutError'));
    }, timeoutMs);

    let fetchSignal = signal;
    if (fetchSignal) {
      const combined = new AbortController();
      const abortCombined = () => combined.abort();
      fetchSignal.addEventListener('abort', abortCombined, { once: true });
      timeoutController.signal.addEventListener('abort', abortCombined, { once: true });
      fetchSignal = combined.signal;
    } else {
      fetchSignal = timeoutController.signal;
    }

    try {
      const resp = await fetch(API.apiFetchUrl('/api/state', queryParams), {
        cache: 'no-store',
        signal: fetchSignal,
      });
      const data = await resp.json().catch(() => ({}));
      if (resp.status === 400 && data.status === 'unavailable') {
        throw new Error(`Timeframe ${data.timeframe} not available`);
      }
      if (resp.status === 503 || data.status === 'warming_up') {
        return { warmingUp: true, data };
      }
      if (!resp.ok) throw new Error(`API error: ${resp.status}`);
      return { warmingUp: false, data };
    } finally {
      clearTimeout(timeoutId);
    }
  },

  async fetchLiveState(options = {}) {
    const {
      tf,
      userTfChange = false,
      navigatorsOnly = false,
      signal = null,
      params = {},
      timeoutMs = API.LIVE_STATE_FETCH_TIMEOUT_MS,
    } = options;

    const key = API.liveStateRequestKey(tf, { navigatorsOnly });

    if (userTfChange) {
      API.abortLiveStateFetch();
      API._liveStateAbort = new AbortController();
    } else if (API._liveStateInflight.has(key)) {
      return API._liveStateInflight.get(key);
    }

    const fetchSignal = userTfChange ? API._liveStateAbort.signal : signal;
    const promise = API.fetchState({
      params,
      navigatorsOnly,
      signal: fetchSignal,
      timeoutMs,
    }).finally(() => {
      if (API._liveStateInflight.get(key) === promise) {
        API._liveStateInflight.delete(key);
      }
    });

    if (!userTfChange) {
      API._liveStateInflight.set(key, promise);
    }
    return promise;
  },

  async fetchPollState({ tf, limit, rsxLookback }) {
    const resp = await fetch(API.apiFetchUrl('/api/state', {
      tf,
      limit,
      poll: '1',
      rsxLookback,
    }), { cache: 'no-store' });
    const data = await resp.json().catch(() => ({}));
    if (resp.status === 400 && data.status === 'unavailable') {
      throw new Error(`Timeframe ${data.timeframe} not available`);
    }
    if (resp.status === 503 || data.status === 'warming_up') {
      return { warmingUp: true, data };
    }
    if (!resp.ok) throw new Error(`API error: ${resp.status}`);
    return { warmingUp: false, data };
  },

  async fetchLiveHistory({ tf, endTimeSec, limit, rsxSettings }) {
    const params = new URLSearchParams({
      tf,
      endTime: String(endTimeSec),
      limit: String(limit),
      rsx_length: String(rsxSettings.length),
      rsx_signal_length: String(rsxSettings.signal_length),
      rsx_source: rsxSettings.source,
      rsx_method: rsxSettings.div_method,
      rsx_pivot_radius: String(rsxSettings.pivot_radius),
      rsx_div_lookback: String(rsxSettings.div_lookback),
      min_price_delta_ratio: String(rsxSettings.min_price_delta_ratio),
      min_osc_delta: String(rsxSettings.min_osc_delta),
    });
    const resp = await fetch(`/api/history?${params.toString()}`, { cache: 'no-store' });
    const data = await resp.json().catch(() => ({}));
    if (!resp.ok) throw new Error(`history API: ${resp.status}`);
    return data;
  },

  async fetchBacktestHistoryChunk(params) {
    const resp = await fetch(`/api/history/chunk?${params.toString()}`, { cache: 'no-store' });
    const data = await resp.json().catch(() => ({}));
    if (!resp.ok) {
      return { ok: false, data };
    }
    return { ok: true, data };
  },

  async fetchRsxSettings() {
    const resp = await fetch('/api/settings/indicators');
    if (!resp.ok) {
      throw new Error(`RSX settings GET failed (${resp.status})`);
    }
    return resp.json();
  },

  async pushRsxSettings(settings) {
    const resp = await fetch('/api/settings/indicators', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(settings),
    });
    if (!resp.ok) {
      const errText = await resp.text().catch(() => '');
      throw new Error(`RSX settings POST failed (${resp.status}): ${errText || 'no body'}`);
    }
    return resp.json();
  },

  async postNavigatorSettings(navigators) {
    const resp = await fetch('/api/settings/navigators', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ navigators }),
    });
    if (!resp.ok) {
      throw new Error(`navigator settings sync failed: ${resp.status}`);
    }
    return resp.json().catch(() => ({}));
  },

  async fetchRiskSettings() {
    const resp = await fetch('/api/settings/risk');
    if (!resp.ok) {
      throw new Error(`risk settings GET failed (${resp.status})`);
    }
    return resp.json();
  },

  async postRiskSettings(payload) {
    const resp = await fetch('/api/settings/risk', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (!resp.ok) {
      throw new Error(`risk settings POST failed (${resp.status})`);
    }
    return resp.json();
  },

  async postThresholds(thresholds) {
    const resp = await fetch('/api/settings/thresholds', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(thresholds),
    });
    if (!resp.ok) {
      throw new Error(`thresholds POST failed (${resp.status})`);
    }
    return resp;
  },

  async postMatrix(matrix) {
    const resp = await fetch('/api/settings/matrix', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(matrix),
    });
    if (!resp.ok) {
      throw new Error(`matrix POST failed (${resp.status})`);
    }
    return resp;
  },

  async fetchStats(mode) {
    const resp = await fetch(`/api/stats?mode=${encodeURIComponent(mode)}`);
    if (!resp.ok) {
      throw new Error(`HTTP ${resp.status}`);
    }
    return resp.json();
  },

  async runBacktest(payload, signal) {
    const resp = await fetch('/api/backtest/run', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
      signal,
    });
    const rawText = await resp.text();
    let result = {};
    try {
      const parsed = rawText ? JSON.parse(rawText) : {};
      result = API.normalizeGoResponse(parsed);
    } catch {
      result = { _parseError: true };
    }
    return { ok: resp.ok, status: resp.status, result, rawText };
  },

  async stopBacktest() {
    const resp = await fetch('/api/backtest/stop', { method: 'POST' });
    if (!resp.ok) {
      throw new Error(`backtest stop failed (${resp.status})`);
    }
    return resp;
  },
};

if (typeof window !== 'undefined') {
  window.API = API;
}
