// CSS only allows @import before every other rule. Vite drops a late
// @import with nothing but a build warning, which is how the reading-
// preference fonts silently went missing from production builds.
import { readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'

const SRC = join(__dirname, '..')

function cssFiles(dir) {
  return readdirSync(dir, { withFileTypes: true }).flatMap((d) => {
    const p = join(dir, d.name)
    if (d.isDirectory()) return d.name === 'node_modules' ? [] : cssFiles(p)
    return d.name.endsWith('.css') ? [p] : []
  })
}

describe('stylesheets', () => {
  it.each(cssFiles(SRC).map((f) => [f.slice(SRC.length + 1), f]))('%s keeps every @import first', (_, file) => {
    const css = readFileSync(file, 'utf8').replace(/\/\*[\s\S]*?\*\//g, '')
    const statements = css.split(/[;{}]/).map((s) => s.trim()).filter(Boolean)
    const firstOther = statements.findIndex((s) => !/^@(import|charset|layer\s+[\w\s,.-]+$)/.test(s))
    const lateImport = statements.findIndex((s, i) => i > firstOther && firstOther !== -1 && s.startsWith('@import'))
    expect(lateImport).toBe(-1)
  })
})
