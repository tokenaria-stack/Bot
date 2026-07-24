/**
 * ADR-021 P2 CrosshairController unit tests (Node).
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
  const log = {
    horzMaps: [],
    syncPeers: [],
    clears: [],
  };
  return {
    log,
    hooks: {
      applyHorzVisibility: (map) => { log.horzMaps.push({ ...map }); },
      syncPeerTime: (sourceHostId, time, param) => {
        log.syncPeers.push({ sourceHostId, time });
      },
      clearPeerCrosshairs: (sourceHostId) => {
        log.clears.push(sourceHostId);
      },
    },
  };
}

test('hover price: only price horz true', () => {
  CrosshairController._resetForTests();
  const { log, hooks } = recordingHooks();
  CrosshairController.bind(hooks);
  CrosshairController.setHovered('price');
  const map = log.horzMaps[log.horzMaps.length - 1];
  assert.strictEqual(map.price, true);
  assert.strictEqual(map.rsx, false);
  assert.strictEqual(map.wozduh, false);
  assert.strictEqual(CrosshairController.getHovered(), 'price');
});

test('hover rsx: only rsx horz true; sync peers with time not source Y API', () => {
  CrosshairController._resetForTests();
  const { log, hooks } = recordingHooks();
  CrosshairController.bind(hooks);
  CrosshairController.onCrosshairMove('rsx', {
    point: { x: 1, y: 2 },
    time: 1000,
    __sourceY: 55,
  });
  assert.strictEqual(CrosshairController.getHovered(), 'rsx');
  const map = log.horzMaps[log.horzMaps.length - 1];
  assert.strictEqual(map.rsx, true);
  assert.strictEqual(map.price, false);
  assert.strictEqual(map.wozduh, false);
  assert.strictEqual(log.syncPeers.length, 1);
  assert.strictEqual(log.syncPeers[0].sourceHostId, 'rsx');
  assert.strictEqual(log.syncPeers[0].time, 1000);
  // Controller does not compute or send a peer Y — ChartAdapter resolves local Y per pane.
  assert.strictEqual(Object.prototype.hasOwnProperty.call(log.syncPeers[0], 'peerY'), false);
});

test('hover wozduh then leave: horz all false, peers cleared', () => {
  CrosshairController._resetForTests();
  const { log, hooks } = recordingHooks();
  CrosshairController.bind(hooks);
  CrosshairController.onCrosshairMove('wozduh', { point: { x: 0, y: 0 }, time: 50 });
  assert.strictEqual(CrosshairController.getHovered(), 'wozduh');
  CrosshairController.onCrosshairMove('wozduh', {}); // leave
  assert.strictEqual(CrosshairController.getHovered(), null);
  const map = log.horzMaps[log.horzMaps.length - 1];
  assert.strictEqual(map.price, false);
  assert.strictEqual(map.rsx, false);
  assert.strictEqual(map.wozduh, false);
  assert.ok(log.clears.includes(null));
});

test('horzVisibilityMap pure policy — exactly one horz when hovered', () => {
  const price = CrosshairController.horzVisibilityMap('price');
  assert.deepStrictEqual(price, { price: true, wozduh: false, rsx: false });
  const rsx = CrosshairController.horzVisibilityMap('rsx');
  assert.deepStrictEqual(rsx, { price: false, wozduh: false, rsx: true });
  const none = CrosshairController.horzVisibilityMap(null);
  assert.deepStrictEqual(none, { price: false, wozduh: false, rsx: false });
});

test('time sync without point does not set hover; null time clears peers', () => {
  CrosshairController._resetForTests();
  const { log, hooks } = recordingHooks();
  CrosshairController.bind(hooks);
  CrosshairController.onCrosshairMove('price', { point: { x: 1, y: 1 }, time: null });
  assert.strictEqual(CrosshairController.getHovered(), 'price');
  assert.ok(log.clears.includes('price'));
  assert.strictEqual(log.syncPeers.length, 0);
});

test('shouldIgnore blocks moves', () => {
  CrosshairController._resetForTests();
  const { log, hooks } = recordingHooks();
  hooks.shouldIgnore = () => true;
  CrosshairController.bind(hooks);
  CrosshairController.onCrosshairMove('rsx', { point: { x: 1, y: 1 }, time: 9 });
  assert.strictEqual(CrosshairController.getHovered(), null);
  assert.strictEqual(log.syncPeers.length, 0);
});

test('invariant: CrosshairController has no timeline API', () => {
  assert.strictEqual(typeof CrosshairController.commit, 'undefined');
  assert.strictEqual(typeof CrosshairController.proposeFromPane, 'undefined');
  assert.strictEqual(typeof CrosshairController.setVisibleLogicalRange, 'undefined');
});

console.log('crosshair_controller_test: ALL PASS');
