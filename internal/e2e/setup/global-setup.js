import { execSync } from 'child_process';
import { existsSync, rmSync } from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(__dirname, '..', '..', '..');
const DB_PATH = path.join(ROOT, 'testdata.db');
const SEED_PATH = path.join(ROOT, 'internal', 'e2e', 'seed', 'main.go');

export default async function globalSetup() {
  if (existsSync(DB_PATH)) {
    rmSync(DB_PATH);
  }
  execSync(`go run ${SEED_PATH} ${DB_PATH}`, {
    cwd: ROOT,
    stdio: 'inherit',
  });
}
