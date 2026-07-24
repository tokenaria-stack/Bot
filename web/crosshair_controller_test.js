/**
 * ADR-021 hover ownership tests (Node).
 * Run: node web/crosshair_controller_test.js
 */
'use strict';

const assert = require('assert');
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
      syncPeerTime: (sourceHostId, time) => {
        log.syncPeers.push({ sourceHostId, time });
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

test('syncTime cannot change hoveredHostId', () => {
  CrosshairController._resetForTests();
  const { hooks } = recordingHooks();
  CrosshairController.bind(hooks);
  CrosshairController.setHovered('wozduh');
  CrosshairController.syncTime({ sourceHostId: 'price', time: 100 });
  assert.strictEqual(CrosshairController.getHovered(), 'wozduh');
  CrosshairController.syncTime({ sourceHostId: 'rsx', time: 200 });
  assert.strictEqual(CrosshairController.getHovered(), 'wozduh');
});

test('peer sync only from hovered source; repeated sync cannot steal hover', () => {
  CrosshairController._resetForTests();
  const { log, hooks } = recordingHooks();
  CrosshairController.bind(hooks);
  CrosshairController.setHovered('rsx');
  for (let i = 0; i < 20; i++) {
    assert.strictEqual(
      CrosshairController.syncTime({ sourceHostId: 'rsx', time: 1000 + i }),
      true,
    );
    // Peer pretending to be active — must not sync and must not steal hover.
    assert.strictEqual(
      CrosshairController.syncTime({ sourceHostId: 'price', time: 1000 + i }),
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
  CrosshairController.syncTime({ sourceHostId: 'price', time: 42 });
  // After sync, policy re-applied — still only price horz.
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

test('no LWC-shaped API on CrosshairController', () => {
  assert.strictEqual(typeof CrosshairController.onCrosshairMove, 'undefined');
  assert.strictEqual(typeof CrosshairController.commit, 'undefined');
  assert.strictEqual(typeof CrosshairController.syncTime.length, 'number');
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

console.log('crosshair_controller_test: ALL PASS');
