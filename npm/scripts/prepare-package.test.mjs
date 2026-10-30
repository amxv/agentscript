import assert from 'node:assert/strict';
import { existsSync, mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';
import { preparePackage } from './prepare-package.mjs';

const repo = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');

test('npm package is separate from the Astro site and repository root', () => {
  assert.equal(existsSync(path.join(repo, 'package.json')), false);
  const docs = JSON.parse(readFileSync(path.join(repo, 'docs/package.json'), 'utf8'));
  assert.equal(docs.private, true);
  assert.ok(docs.devDependencies.astro);
});

test('npm stage includes runnable CLI with Go fallback but excludes docs and test data', (t) => {
  const root = mkdtempSync(path.join(tmpdir(), 'cli-npm-stage-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const output = path.join(root, 'stage');
  preparePackage(output, '9.8.7');
  const pkg = JSON.parse(readFileSync(path.join(output, 'package.json'), 'utf8'));
  assert.equal(pkg.version, '9.8.7');
  assert.deepEqual(pkg.scripts, { postinstall: 'node scripts/postinstall.js' });
  for (const item of ['go.mod', 'README.md', 'LICENSE', `bin/${pkg.config.cliBinaryName}.js`, 'scripts/postinstall.js', `cmd/${pkg.config.cliBinaryName}/main.go`]) {
    assert.ok(existsSync(path.join(output, item)), `missing ${item}`);
  }
  for (const item of ['docs', 'astro.config.mjs', 'bun.lock', 'src', 'package-lock.json']) {
    assert.equal(existsSync(path.join(output, item)), false, `${item} leaked into npm`);
  }
});
