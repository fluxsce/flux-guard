// 按当前系统调用 plugins/<name>/build.ps1 或 build.sh，打出清单要求的探针。
import { spawnSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const allowed = ["kafka", "zk"];
const names = process.argv.slice(2);
const targets = names.length === 0 ? allowed : names;
for (const name of targets) {
  if (!allowed.includes(name)) {
    console.error("插件名只能是 kafka 或 zk");
    process.exit(1);
  }
}

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const win = process.platform === "win32";

for (const name of targets) {
  const script = path.join(root, "plugins", name, win ? "build.ps1" : "build.sh");
  const command = win ? "powershell" : "bash";
  const args = win
    ? ["-NoProfile", "-ExecutionPolicy", "Bypass", "-File", script]
    : [script];
  const result = spawnSync(command, args, { stdio: "inherit", cwd: root });
  if (result.error) {
    console.error(result.error.message);
    process.exit(1);
  }
  if (result.status !== 0) {
    process.exit(result.status ?? 1);
  }
  const base = `${name}-probe`;
  const exe = path.join(root, "plugins", name, "dist", base, win ? `${base}.exe` : base);
  if (!fs.existsSync(exe)) {
    console.error(`没有打出探针: plugins/${name}/dist/${base}/${win ? `${base}.exe` : base}`);
    process.exit(1);
  }
}
