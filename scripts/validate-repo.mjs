import { execFileSync } from 'node:child_process';
import { readdirSync, statSync } from 'node:fs';
import { join } from 'node:path';

const root = process.cwd();
const skipDirs = new Set(['.git', 'node_modules', 'dist', 'build', 'target', '.next']);
const fullValidation = process.env.FULL_VALIDATION === '1';

function walk(dir, matcher, out = []) {
  for (const entry of readdirSync(dir)) {
    if (skipDirs.has(entry)) continue;
    const full = join(dir, entry);
    const stat = statSync(full);
    if (stat.isDirectory()) walk(full, matcher, out);
    else if (matcher(full)) out.push(full);
  }
  return out;
}

function run(label, command, args, cwd = root) {
  console.log(`\n==> ${label}`);
  execFileSync(command, args, { cwd, stdio: 'inherit' });
}

run('TypeScript root config', 'npx', ['tsc', '-p', 'tsconfig.json', '--noEmit']);

const goMods = walk(root, (file) => file.endsWith('/go.mod'));
const cargoTomls = walk(root, (file) => file.endsWith('/Cargo.toml'));
console.log(`\nDiscovered ${goMods.length} Go modules and ${cargoTomls.length} Rust manifests.`);

if (!fullValidation) {
  console.log('Set FULL_VALIDATION=1 to run Go/Rust/Solidity ecosystem checks. They may require external module registries.');
  process.exit(0);
}

for (const mod of goMods) {
  run(`Go tests: ${mod.replace(root + '/', '')}`, 'go', ['test', './...'], mod.slice(0, -'/go.mod'.length));
}

for (const manifest of cargoTomls) {
  run(`Rust check: ${manifest.replace(root + '/', '')}`, 'cargo', ['check', '--locked', '--offline', '--manifest-path', manifest]);
}

run('Solidity compile', 'npm', ['run', 'compile'], join(root, 'smart_contracts/evm_contracts'));
