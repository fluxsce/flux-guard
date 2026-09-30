import { readFileSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = join(dirname(fileURLToPath(import.meta.url)), '..')
const pkg = JSON.parse(readFileSync(join(root, 'package.json'), 'utf8'))
const version = String(pkg.version ?? '').trim()
if (!/^\d+\.\d+\.\d+$/.test(version)) {
  throw new Error(`package.json version must be x.y.z, got ${JSON.stringify(pkg.version)}`)
}

const dest = join(root, 'common/buildinfo/package_version.go')
const body = `package buildinfo

// PackageVersion 来自仓库根 package.json。改版本后执行 node scripts/sync-version.mjs。
const PackageVersion = "${version}"
`
writeFileSync(dest, body, 'utf8')
process.stdout.write(`PackageVersion ${version}\n`)
