#!/usr/bin/env node
// Assemble a standalone npm package while leaving the Go module at repo root.
// The Go source is required when a release download fails during postinstall.
import { cpSync, existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const npmRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const repoRoot = path.resolve(npmRoot, '..');
const excluded = /(?:^|[/\\])testdata(?:[/\\]|$)|_test\.go$|\.(?:db|eml|key|p12|pem|pfx|sqlite|sqlite3)$/i;

export function preparePackage(outputDir = path.join(npmRoot, '.publish'), version) {
  const manifest = JSON.parse(readFileSync(path.join(npmRoot, 'package.json'), 'utf8'));
  if (version) manifest.version = version;
  // Installers should run only the postinstall hook, not repository-only scripts.
  manifest.scripts = { postinstall: manifest.scripts.postinstall };
  delete manifest.devDependencies;
  delete manifest.packageManager;

  rmSync(outputDir, { recursive: true, force: true });
  mkdirSync(outputDir, { recursive: true });
  for (const item of manifest.files) {
    const from = (item.startsWith('bin/') || item.startsWith('scripts/'))
      ? path.join(npmRoot, item) : path.join(repoRoot, item);
    if (!existsSync(from)) throw new Error(`Missing npm package input: ${from}`);
    const dest = path.join(outputDir, item);
    mkdirSync(path.dirname(dest), { recursive: true });
    cpSync(from, dest, { recursive: true, filter: (source) => !excluded.test(source) });
  }
  writeFileSync(path.join(outputDir, 'package.json'), `${JSON.stringify(manifest, null, 2)}\n`);
  return outputDir;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  console.log(`Prepared npm distribution at ${preparePackage(undefined, process.argv[2])}`);
}
