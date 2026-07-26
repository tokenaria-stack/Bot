/**
 * Wave 1 invariant — source contracts (no LWC).
 * Data never changes VIEW; only TimeCamera decides navigation.
 */
const assert = require('assert');
const fs = require('fs');
const path = require('path');

const boot = fs.readFileSync(path.join(__dirname, 'boot.js'), 'utf8');
const compositor = fs.readFileSync(path.join(__dirname, 'chart-compositor.js'), 'utf8');
const timeCamera = fs.readFileSync(path.join(__dirname, 'ui/time-camera.js'), 'utf8');

function extractFn(src, name) {
  const re = new RegExp(`function ${name}\\s*\\([^)]*\\)\\s*\\{`);
  const m = src.match(re);
  assert.ok(m, `missing function ${name}`);
  const start = m.index + m[0].length - 1;
  let depth = 0;
  for (let i = start; i < src.length; i++) {
    if (src[i] === '{') depth += 1;
    else if (src[i] === '}') {
      depth -= 1;
      if (depth === 0) return src.slice(m.index, i + 1);
    }
  }
  assert.fail(`unclosed ${name}`);
}

const returnToLive = extractFn(boot, 'maybeReturnToLiveFromHistory');
assert.ok(!/loadDashboard\s*\(/.test(returnToLive),
  'Wave1 gate: maybeReturnToLiveFromHistory must not call loadDashboard');
assert.ok(!/windowMode/.test(returnToLive),
  'Wave1 gate: maybeReturnToLiveFromHistory must not read windowMode for navigation');

assert.ok(!/_commitPrependCamera/.test(compositor),
  'Wave1 gate: Compositor must not own _commitPrependCamera policy');
assert.ok(/proposePreserveViewport/.test(compositor),
  'Wave1 gate: Compositor must publish facts via TimeCamera.proposePreserveViewport');
assert.ok(/_publishPrependViewportFacts/.test(compositor),
  'Wave1 gate: prepend path must use fact publisher');

assert.ok(/function proposePreserveViewport/.test(timeCamera),
  'Wave1 gate: TimeCamera owns proposePreserveViewport');

// Heal/System may still call loadDashboard — but not from the Data return-to-live function.
assert.ok(/loadDashboard\s*\(/.test(boot), 'System/User hydrate paths still exist');

console.log('wave1_invariant_test: ALL PASS');
