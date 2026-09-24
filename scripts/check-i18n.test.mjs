import assert from 'node:assert/strict';
import { mkdtemp, mkdir, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { spawnSync } from 'node:child_process';
import test from 'node:test';

const validator = resolve(import.meta.dirname, 'check-i18n.mjs');

const complete = {
  'greeting': 'Hello {{name}}',
  'items_one': '{{count}} item',
  'items_other': '{{count}} items'
};

async function fixture({ en = complete, enRaw, es = complete, esRaw, source } = {}) {
  const root = await mkdtemp(join(tmpdir(), 'hourpaths-i18n-'));
  await mkdir(join(root, 'packages/i18n/src/locales'), { recursive: true });
  await mkdir(join(root, 'apps/web/src'), { recursive: true });
  await mkdir(join(root, 'apps/mobile'), { recursive: true });
  await writeFile(join(root, 'packages/i18n/src/locales/en.json'), enRaw ?? JSON.stringify(en));
  await writeFile(join(root, 'packages/i18n/src/locales/es.json'), esRaw ?? JSON.stringify(es));
  if (source) await writeFile(join(root, 'apps/web/src/fixture.svelte'), source);
  return root;
}

function validate(root) {
  return spawnSync(process.execPath, [validator, '--root', root], { encoding: 'utf8' });
}

async function rejects(name, mutation, expected) {
  await test(name, async () => {
    const root = await fixture(mutation);
    try {
      const result = validate(root);
      assert.notEqual(result.status, 0, result.stdout);
      assert.match(result.stderr, expected);
    } finally {
      await rm(root, { recursive: true, force: true });
    }
  });
}

test('accepts complete catalogs without literal presentation copy', async () => {
  const root = await fixture({ source: '<p>{translator.t("greeting", { name })}</p>' });
  try {
    const result = validate(root);
    assert.equal(result.status, 0, result.stderr);
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

await rejects('rejects an empty reference catalog', { en: {} }, /en\.json catalog must not be empty/);
await rejects('rejects an empty translated catalog', { es: {} }, /es\.json catalog must not be empty/);
await rejects('rejects a non-object catalog', { es: [] }, /es\.json catalog must be a JSON object/);
await rejects('rejects a null catalog without crashing', { es: null }, /es\.json catalog must be a JSON object/);
await rejects(
  'rejects duplicate catalog keys before JSON parsing can discard them',
  { esRaw: '{"greeting":"Hola","greeting":"Otra vez","items_one":"Uno","items_other":"Muchos"}' },
  /es\.json has duplicate key greeting/,
);
await rejects('rejects missing catalog keys', { es: { ...complete, greeting: undefined } }, /es\.json is missing greeting/);
await rejects('rejects extra catalog keys', { es: { ...complete, extra: 'Extra' } }, /es\.json has unexpected extra/);
await rejects('rejects empty messages', { es: { ...complete, greeting: ' ' } }, /es\.json has an empty or non-string greeting/);
await rejects('rejects interpolation mismatch', { es: { ...complete, greeting: 'Hola {{person}}' } }, /interpolation parameters differ for greeting/);
await rejects('rejects missing plural forms', { en: { greeting: 'Hello', items_one: 'One' }, es: { greeting: 'Hola', items_one: 'Uno' } }, /plural items_one requires items_other/);
await rejects('rejects orphan plural forms', { en: { greeting: 'Hello', items_other: 'Many' }, es: { greeting: 'Hola', items_other: 'Muchos' } }, /plural items_other requires items_one/);
await rejects('rejects literal presentation copy', { source: '<button aria-label="Save changes">Save changes</button>' }, /contains literal UI copy: Save changes/);
await rejects('rejects literal translatable attributes', { source: '<button aria-label="Save changes">{translator.t("greeting")}</button>' }, /contains literal UI attribute copy: Save changes/);
await rejects('rejects literal expression copy', { source: '<p>{"Ready now"}</p>' }, /contains literal expression copy: Ready now/);
await rejects('rejects literal presentation variables', { source: '<script>const label = "Ready now";</script><p>{label}</p>' }, /contains literal copy variable: Ready now/);
