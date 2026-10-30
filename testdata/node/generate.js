// Regenerates the *.json goldens in this directory from the Node
// implementation of dotenvx, @dotenvx/primitives:
//
//   node generate.js path/to/node_modules/@dotenvx/primitives
//
// Every NAME.env is parsed twice with MACHINE=machine as the only existing
// environment variable and no key files: NAME.json without overload and
// NAME.overload.json with it.
const fs = require('fs')
const path = require('path')

const primitives = require(path.resolve(process.argv[2]))
const version = require(path.resolve(process.argv[2], 'package.json')).version

for (const file of fs.readdirSync(__dirname).filter(f => f.endsWith('.env')).sort()) {
  const src = fs.readFileSync(path.join(__dirname, file), 'utf8')
  for (const overload of [false, true]) {
    const { parsed } = primitives.parseSync(src, { processEnv: { MACHINE: 'machine' }, fk: [], overload })
    const out = path.join(__dirname, file.replace(/\.env$/, overload ? '.overload.json' : '.json'))
    fs.writeFileSync(out, JSON.stringify(parsed, null, 2) + '\n')
  }
}
console.log(`generated with @dotenvx/primitives ${version}`)
