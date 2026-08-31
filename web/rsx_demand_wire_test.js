/**
 * DAG-DEMAND-1 — WS subscribe carries explicit facts (including []).
 * Run: node web/rsx_demand_wire_test.js
 */
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');

const ws = fs.readFileSync(path.join(__dirname, 'ws.js'), 'utf8');
assert.ok(ws.includes('_facts'), 'WS retains facts list');
assert.ok(ws.includes('payload.facts = WS._facts'), 'explicit facts array is sent');
assert.ok(ws.includes('Array.isArray(facts)'), 'subscribe copies facts when provided');

const boot = fs.readFileSync(path.join(__dirname, 'boot.js'), 'utf8');
assert.ok(boot.includes('resolveLiveFactIds'), 'boot derives fact demand');
assert.ok(boot.includes('WS.subscribe(tf, tf, slots, facts)'), 'subscribe sends facts');

const ctrl = fs.readFileSync(path.join(__dirname, 'ui/rsx-controller.js'), 'utf8');
assert.ok(ctrl.includes('wsSubscribeTf(window.currentTf)'), 'visibility change updates demand');
assert.ok(ctrl.includes('rsx-vis-chk'), 'visibility checkboxes remain the UI');

console.log('rsx_demand_wire_test.js passed');
