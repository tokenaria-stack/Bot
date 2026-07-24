/**
 * ADR-022 ScaleContribution unit tests (Node).
 * Run: node web/scale_contribution_test.js
 */
'use strict';

const assert = require('assert');
const { createAutoscaleProvider } = require('./ui/scale-contribution.js');
const { DDRFactory } = require('./series-factory.js');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

test('dynamic / omit / unknown → undefined provider', () => {
  assert.strictEqual(createAutoscaleProvider(undefined), undefined);
  assert.strictEqual(createAutoscaleProvider(null), undefined);
  assert.strictEqual(createAutoscaleProvider({}), undefined);
  assert.strictEqual(createAutoscaleProvider({ type: 'dynamic' }), undefined);
  assert.strictEqual(createAutoscaleProvider({ type: 'symmetric' }), undefined);
});

test('bounded → fixed priceRange provider', () => {
  const p = createAutoscaleProvider({ type: 'bounded', min: -5, max: 105 });
  assert.strictEqual(typeof p, 'function');
  assert.deepStrictEqual(p(), {
    priceRange: { minValue: -5, maxValue: 105 },
  });
});

test('bounded invalid min/max → undefined (fail closed to dynamic)', () => {
  assert.strictEqual(createAutoscaleProvider({ type: 'bounded', min: 5, max: 5 }), undefined);
  assert.strictEqual(createAutoscaleProvider({ type: 'bounded', min: 10, max: 1 }), undefined);
  assert.strictEqual(createAutoscaleProvider({ type: 'bounded', min: 'x', max: 105 }), undefined);
  assert.strictEqual(createAutoscaleProvider({ type: 'bounded' }), undefined);
});

test('ignore → null provider', () => {
  const p = createAutoscaleProvider({ type: 'ignore' });
  assert.strictEqual(typeof p, 'function');
  assert.strictEqual(p(), null);
});

test('SeriesFactory mounts bounded / ignore from renderOptions.scaleContribution', () => {
  const captured = [];
  const fakeChart = {
    addLineSeries(opts) {
      captured.push(opts);
      return {
        priceScale() {
          return { applyOptions() {} };
        },
      };
    },
  };
  const factory = new DDRFactory();
  factory.buildPanes(
    { rsx: { chart: fakeChart } },
    {
      pane_osc: [
        {
          id: 'line_rsx',
          hostId: 'rsx',
          kind: 'line',
          renderOptions: {
            title: 'RSX',
            scaleContribution: { type: 'bounded', min: -5, max: 105 },
          },
        },
        {
          id: 'line_rsx_signal',
          hostId: 'rsx',
          kind: 'line',
          renderOptions: {
            title: 'RSX Signal',
            scaleContribution: { type: 'ignore' },
          },
        },
        {
          id: 'line_plain',
          hostId: 'rsx',
          kind: 'line',
          renderOptions: { title: 'plain' },
        },
      ],
    },
  );

  assert.strictEqual(captured.length, 3);
  assert.ok(!Object.prototype.hasOwnProperty.call(captured[0], 'scaleContribution'));
  assert.strictEqual(typeof captured[0].autoscaleInfoProvider, 'function');
  assert.deepStrictEqual(captured[0].autoscaleInfoProvider(), {
    priceRange: { minValue: -5, maxValue: 105 },
  });
  assert.strictEqual(captured[1].autoscaleInfoProvider(), null);
  assert.strictEqual(captured[2].autoscaleInfoProvider, undefined);
});

console.log('scale_contribution_test: ALL PASS');
