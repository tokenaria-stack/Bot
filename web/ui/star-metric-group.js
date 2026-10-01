/**
 * Shared father/child metric rows.
 * Three rows per line: the line, slope, acceleration. No value child.
 * Column 2 is the triangle on the father and the color square on the child.
 */
const StarMetricGroup = (() => {
  function escapeText(value) {
    return String(value == null ? '' : value)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/"/g, '&quot;');
  }

  function classifyLeaf(leaf) {
    if (/Accel$/.test(leaf)) return { role: 'accel', stem: leaf.replace(/Accel$/, '') };
    if (/Slope$/.test(leaf)) return { role: 'slope', stem: leaf.replace(/Slope$/, '') };
    if (leaf === 'rsxMinus') return { role: 'minus', stem: 'rsx' };
    return { role: 'level', stem: leaf };
  }

  function splitId(id) {
    const raw = String(id || '');
    const i = raw.lastIndexOf('.');
    if (i < 0) return { prefix: '', leaf: raw };
    return { prefix: raw.slice(0, i + 1), leaf: raw.slice(i + 1) };
  }

  function childLabel(role) {
    if (role === 'slope') return 'slope';
    if (role === 'accel') return 'acceleration';
    if (role === 'minus') return 'minus signal';
    return role;
  }

  function bundleReadings(readings) {
    const order = [];
    const map = {};
    (readings || []).forEach((reading) => {
      const parts = splitId(reading.id);
      const cls = classifyLeaf(parts.leaf);
      const key = parts.prefix + cls.stem;
      if (!map[key]) {
        map[key] = { key, prefix: parts.prefix, stem: cls.stem, group: reading.group, level: null, slope: null, accel: null, extras: [] };
        order.push(key);
      }
      const g = map[key];
      if (cls.role === 'level') g.level = reading;
      else if (cls.role === 'slope') g.slope = reading;
      else if (cls.role === 'accel') g.accel = reading;
      else g.extras.push({ role: cls.role, reading });
    });
    return order.map((key) => map[key]);
  }

  const CATALOG_LINES = [
    { parent: 'Vwema', slope: 'Slope', accel: 'VwemaAccel', label: 'VWEMA' },
    { parent: 'ChanMid', slope: 'MidSlope', accel: 'MidAccel', label: 'Orange midline' },
    { parent: 'Ema5', slope: 'Ema5Slope', accel: 'Ema5Accel', label: 'Volume RSI EMA5' },
    { parent: 'Ema12', slope: 'Ema12Slope', accel: '', label: 'Volume RSI EMA12' },
    { parent: 'RsiClose', slope: 'RsiCloseSlope', accel: 'RsiCloseAccel', label: 'RSI close' },
    { parent: 'Ema7', slope: 'Ema7Slope', accel: 'Ema7Accel', label: 'RSI EMA7' },
    { parent: 'Macd', slope: 'MacdSlope', accel: 'MacdAccel', label: 'MACD RSI close' },
  ];

  const TF = [
    { prefix: 'M15.', tf: '15m' },
    { prefix: 'H1.', tf: '1h' },
    { prefix: 'H4.', tf: '4h' },
    { prefix: 'D1.', tf: 'Daily' },
  ];

  function catalogIndex(entries) {
    const map = {};
    (entries || []).forEach((e) => { map[e.id] = e; });
    return map;
  }

  function take(map, id) {
    const e = map[id];
    if (e) delete map[id];
    return e || null;
  }

  function bundleCatalog(entries) {
    const map = catalogIndex(entries);
    const sections = [];
    TF.forEach((tf) => {
      const groups = [];
      CATALOG_LINES.forEach((line) => {
        const level = take(map, tf.prefix + line.parent);
        if (!level) return;
        groups.push({
          key: tf.prefix + line.parent,
          label: line.label,
          tf: tf.tf,
          source: 'schema3',
          level,
          slope: line.slope ? take(map, tf.prefix + line.slope) : null,
          accel: line.accel ? take(map, tf.prefix + line.accel) : null,
          extras: [],
        });
      });
      const rsx = rsxBundle(map, tf);
      if (rsx) groups.push(rsx);
      leftoverSingles(map, tf.prefix, tf.tf).forEach((g) => groups.push(g));
      const sig = rsxSignalBundle(map, tf);
      if (sig) groups.push(sig);
      if (groups.length) sections.push({ title: tf.tf, groups });
    });
    const rsxRoot = rootRsxBundle(map);
    if (rsxRoot.groups.length) sections.push({ title: '15m RSX', groups: rsxRoot.groups });
    leftoverGlobal(map).forEach((sec) => sections.push(sec));
    return sections;
  }

  function rsxBundle(map, tf) {
    if (tf.prefix === 'M15.') return null;
    const pfx = tf.prefix.replace('.', '') + 'RSX.';
    const level = take(map, pfx + 'Value');
    if (!level) return null;
    return {
      key: pfx + 'Value',
      label: 'RSX',
      tf: tf.tf,
      source: 'schema3',
      level,
      slope: take(map, pfx + 'Slope'),
      accel: take(map, pfx + 'Accel'),
      extras: [],
    };
  }

  function rootRsxBundle(map) {
    const groups = [];
    const level = take(map, 'RSX');
    if (level) {
      groups.push({
        key: 'RSX',
        label: 'RSX',
        tf: '15m',
        source: 'schema3',
        level,
        slope: take(map, 'RSXSlope'),
        accel: take(map, 'RSXAccel'),
        extras: [],
      });
    }
    const sig = take(map, 'Signal');
    if (sig) {
      groups.push({
        key: 'Signal',
        label: 'RSX signal',
        tf: '15m',
        source: 'schema3',
        level: sig,
        slope: take(map, 'SignalSlope'),
        accel: null,
        extras: [],
      });
    }
    const minus = take(map, 'RSXMinusSignal');
    if (minus) {
      groups.push({
        key: 'RSXMinusSignal',
        label: 'RSX − signal',
        tf: '15m',
        source: 'schema3',
        level: minus,
        slope: null,
        accel: null,
        extras: [],
      });
    }
    leftoverSingles(map, 'H1RSX.', '1h').forEach((g) => groups.push(g));
    leftoverSingles(map, 'H4RSX.', '4h').forEach((g) => groups.push(g));
    leftoverSingles(map, 'D1RSX.', 'Daily').forEach((g) => groups.push(g));
    return { groups };
  }

  function rsxSignalBundle(map, tf) {
    if (tf.prefix === 'M15.') return null;
    const pfx = tf.prefix.replace('.', '') + 'RSX.';
    const sig = take(map, pfx + 'Signal');
    if (!sig) return null;
    return {
      key: pfx + 'Signal',
      label: 'RSX signal',
      tf: tf.tf,
      source: 'schema3',
      level: sig,
      slope: take(map, pfx + 'SignalSlope'),
      accel: null,
      extras: [],
    };
  }

  function leftoverSingles(map, prefix, tf) {
    const out = [];
    Object.keys(map).forEach((id) => {
      if (prefix && id.indexOf(prefix) !== 0) return;
      const e = take(map, id);
      if (!e) return;
      out.push({
        key: id,
        label: e.label.replace(/^\d+[mh] /, '').replace(/^Daily /, ''),
        tf,
        source: e.group === 'Matrix' ? 'matrix' : 'schema3',
        level: e,
        slope: null,
        accel: null,
        extras: [],
      });
    });
    return out;
  }

  function leftoverGlobal(map) {
    const bySection = {};
    const order = [];
    Object.keys(map).forEach((id) => {
      const e = map[id];
      const title = (e.group || 'Other') + ' / ' + (e.section || '');
      if (!bySection[title]) {
        bySection[title] = [];
        order.push(title);
      }
      bySection[title].push({
        key: id,
        label: e.label,
        tf: e.timeframe || '',
        source: e.group === 'Matrix' ? 'matrix' : 'schema3',
        level: e,
        slope: null,
        accel: null,
        extras: [],
      });
      delete map[id];
    });
    return order.map((title) => ({ title, groups: bySection[title] }));
  }

  function rowCount(g) {
    let n = 1;
    if (g.slope) n += 1;
    if (g.accel) n += 1;
    n += (g.extras || []).length;
    return n;
  }

  function canExpand(g) {
    return !!(g && (g.slope || g.accel || (g.extras && g.extras.length)));
  }

  function memberIds(g) {
    const ids = [];
    if (g.level && g.level.id) ids.push(g.level.id);
    if (g.slope && g.slope.id) ids.push(g.slope.id);
    if (g.accel && g.accel.id) ids.push(g.accel.id);
    (g.extras || []).forEach((x) => {
      const r = x.reading || x;
      if (r && r.id) ids.push(r.id);
    });
    return ids;
  }

  return {
    bundleReadings, bundleCatalog, childLabel, classifyLeaf, splitId, escapeText, rowCount, canExpand, memberIds,
  };
})();

if (typeof module !== 'undefined' && module.exports) {
  module.exports = StarMetricGroup;
}
