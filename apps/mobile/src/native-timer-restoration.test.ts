import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { URL } from 'node:url';
import ts from 'typescript';

// Execute the actual shell cleanup effect with a controlled native surface port.
// This catches startup revocation without mounting unrelated network/UI services.
const source = readFileSync(new URL('../app/index.tsx', import.meta.url), 'utf8');
const file = ts.createSourceFile('index.tsx', source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
const effects: ts.CallExpression[] = [];
function visit(node: ts.Node) {
  if (ts.isCallExpression(node) && node.expression.getText(file) === 'useEffect'
      && node.arguments[0]?.getText(file).includes('if (!session) void timerSurfaces.clear()')) effects.push(node);
  ts.forEachChild(node, visit);
}
visit(file);
assert.equal(effects.length, 1);
const effect = effects[0];
const callback = effect.arguments[0].getText(file);
const dependencies = effect.arguments[1].getText(file);

test('restoring a saved session preserves native activity; confirmed absence and sign-out clear it', () => {
  let clears = 0;
  const surface = { clear: async () => { clears++; } };
  let previous: unknown[] | undefined;
  function render(ready: boolean, session: object | null) {
    const deps = new Function('ready', 'session', `return ${dependencies}`)(ready, session) as unknown[];
    if (!previous || deps.some((value, index) => value !== previous![index])) {
      new Function('ready', 'session', 'timerSurfaces', `return (${callback})()`)(ready, session, surface);
    }
    previous = deps;
  }
  render(false, null);
  assert.equal(clears, 0, 'Cold startup must not revoke the persisted native lease');
  const credential = {};
  render(true, credential);
  assert.equal(clears, 0);
  render(true, null);
  assert.equal(clears, 1, 'Sign-out must still clean up');
  previous = undefined; clears = 0;
  render(false, null);
  render(true, null);
  assert.equal(clears, 1, 'Settled restoration without a credential must clean up');
});
