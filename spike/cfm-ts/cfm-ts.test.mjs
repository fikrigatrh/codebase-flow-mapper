// Compares the graph for testdata/nest-di with the hand-written nest-di.expected.txt.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import { test } from 'node:test';
import { analyze } from './cfm-ts.mjs';

const fixture = new URL('./testdata/nest-di/', import.meta.url).pathname;

test('nest-di edges match the hand-written expectation', () => {
  const g = analyze(fixture);
  const byId = new Map(g.nodes.map((n) => [n.id, n]));
  const label = (n) => {
    if (n.kind === 'endpoint') return `endpoint ${n.name}`;
    if (n.kind === 'external') return `external ${n.name}`;
    if (n.kind === 'unresolved') return `unresolved ${n.name}`;
    return n.receiver ? `${n.receiver}.${n.name.split('.').pop()}` : n.name;
  };
  const got = g.edges.map((e) => `${label(byId.get(e.source))} -> ${label(byId.get(e.target))} [${e.method} ${e.confidence}]`);
  const want = fs.readFileSync(fixture + 'nest-di.expected.txt', 'utf8').split('\n').filter((l) => l && !l.startsWith('#'));
  for (const w of want) assert.ok(got.includes(w), `missing: ${w}\ngot:\n${got.join('\n')}`);
  for (const l of got) assert.ok(!/-> (MemoryTxRepo|SmsNotifier)\./.test(l), `decoy is a call target: ${l}`);
  const extra = got.filter((l) => !want.includes(l));
  if (extra.length) console.log('edges not in the expectation:\n  ' + extra.join('\n  '));
});
