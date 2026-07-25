/**
 * ADR-021 / ADR-026 CrosshairController tests (Node).
 * Run: node web/crosshair_controller_test.js
 */
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');
const CrosshairController = require('./ui/crosshair-controller.js');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

function recordingHooks() {
  const log = { horzMaps: [], syncPeers: [], clears: [] };
  return {
    log,
    hooks: {
      applyHorzVisibility: (map) => { log.horzMaps.push({ ...map }); },
      syncPeerCrosshair: (sourceHostId, pos) => {
        log.syncPeers.push({ sourceHostId, ...pos });
      },
      clearPeerCrosshairs: (sourceHostId) => {
        log.clears.push(sourceHostId);
      },
    },
  };
}

test('hoveredHostId changes only from setHovered (pointer path)', () => {
  CrosshairController._resetForTests();
  const { log, hooks } = recordingHooks();
  CrosshairController.bind(hooks);
  assert.strictEqual(CrosshairController.getHovered(), null);
  CrosshairController.setHovered('rsx');
  assert.strictEqual(CrosshairController.getHovered(), 'rsx');
  const map = log.horzMaps[log.horzMaps.length - 1];
  assert.strictEqual(map.rsx, true);
  assert.strictEqual(map.price, false);
  assert.strictEqual(map.wozduh, false);
});

test('syncPosition cannot change hoveredHostId', () => {
  CrosshairController._resetForTests();
  const { hooks } = recordingHooks();
  CrosshairController.bind(hooks);
  CrosshairController.setHovered('wozduh');
  CrosshairController.syncPosition({ sourceHostId: 'price', logical: 10, time: 100 });
  assert.strictEqual(CrosshairController.getHovered(), 'wozduh');
  CrosshairController.syncPosition({ sourceHostId: 'rsx', logical: 20, time: 200 });
  assert.strictEqual(CrosshairController.getHovered(), 'wozduh');
});

test('peer sync only from hovered source; repeated sync cannot steal hover', () => {
  CrosshairController._resetForTests();
  const { log, hooks } = recordingHooks();
  CrosshairController.bind(hooks);
  CrosshairController.setHovered('rsx');
  for (let i = 0; i < 20; i++) {
    assert.strictEqual(
      CrosshairController.syncPosition({ sourceHostId: 'rsx', logical: 100 + i, time: 1000 + i }),
      true,
    );
    assert.strictEqual(
      CrosshairController.syncPosition({ sourceHostId: 'price', logical: 100 + i, time: 1000 + i }),
      false,
    );
    assert.strictEqual(CrosshairController.getHovered(), 'rsx');
  }
  assert.strictEqual(log.syncPeers.length, 20);
  assert.ok(log.syncPeers.every((p) => p.sourceHostId === 'rsx'));
  const map = CrosshairController.horzVisibilityMap(CrosshairController.getHovered());
  assert.strictEqual(map.rsx, true);
  assert.strictEqual(map.price, false);
  assert.strictEqual(map.wozduh, false);
});

test('horizontal line policy never migrates while hover fixed', () => {
  CrosshairController._resetForTests();
  const { log, hooks } = recordingHooks();
  CrosshairController.bind(hooks);
  CrosshairController.setHovered('price');
  CrosshairController.syncPosition({ sourceHostId: 'price', logical: 5, time: 42 });
  const map = log.horzMaps[log.horzMaps.length - 1];
  assert.strictEqual(map.price, true);
  assert.strictEqual(map.rsx, false);
  assert.strictEqual(map.wozduh, false);
});

test('leave hover clears peers and horz', () => {
  CrosshairController._resetForTests();
  const { log, hooks } = recordingHooks();
  CrosshairController.bind(hooks);
  CrosshairController.setHovered('rsx');
  CrosshairController.setHovered(null);
  assert.strictEqual(CrosshairController.getHovered(), null);
  const map = log.horzMaps[log.horzMaps.length - 1];
  assert.deepStrictEqual(map, { price: false, wozduh: false, rsx: false });
  assert.ok(log.clears.includes(null));
});

test('ADR-026: time null + finite logical still syncs peers (empty space)', () => {
  CrosshairController._resetForTests();
  const { log, hooks } = recordingHooks();
  CrosshairController.bind(hooks);
  CrosshairController.setHovered('price');
  assert.strictEqual(
    CrosshairController.syncPosition({ sourceHostId: 'price', logical: 3078.5, time: null }),
    true,
  );
  assert.strictEqual(log.clears.length, 0);
  assert.deepStrictEqual(log.syncPeers, [
    { sourceHostId: 'price', logical: 3078.5, time: null },
  ]);
});

test('ADR-026: logical null clears peers (not time null)', () => {
  CrosshairController._resetForTests();
  const { log, hooks } = recordingHooks();
  CrosshairController.bind(hooks);
  CrosshairController.setHovered('price');
  assert.strictEqual(
    CrosshairController.syncPosition({ sourceHostId: 'price', logical: NaN, time: 99 }),
    false,
  );
  assert.ok(log.clears.includes('price'));
  assert.strictEqual(log.syncPeers.length, 0);
});

test('no LWC-shaped API on CrosshairController', () => {
  assert.strictEqual(typeof CrosshairController.onCrosshairMove, 'undefined');
  assert.strictEqual(typeof CrosshairController.commit, 'undefined');
  assert.strictEqual(typeof CrosshairController.syncPosition.length, 'number');
});

test('horzVisibilityMap pure — exactly one horz when hovered', () => {
  assert.deepStrictEqual(
    CrosshairController.horzVisibilityMap('price'),
    { price: true, wozduh: false, rsx: false },
  );
  assert.deepStrictEqual(
    CrosshairController.horzVisibilityMap(null),
    { price: false, wozduh: false, rsx: false },
  );
});

test('chart-core ADR-026: extract logical + single applyPeerCrosshair path', () => {
  const src = fs.readFileSync(path.join(__dirname, 'chart-core.js'), 'utf8');
  assert.ok(src.includes('extractCrosshairPosition'));
  assert.ok(src.includes('applyPeerCrosshair'));
  assert.ok(src.includes('peer-crosshair-guide'));
  assert.ok(src.includes('logicalToCoordinate'));
  assert.ok(!/invent|extrapola/i.test(src.split('function applyPeerCrosshair')[1]?.slice(0, 1200) || ''));
  assert.ok(src.includes('Never invent') || src.includes('never invent'), 'no fake time comment');
});

console.log('crosshair_controller_test: ALL PASS');
