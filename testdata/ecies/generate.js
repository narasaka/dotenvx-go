// Writes vectors.json: values encrypted by @dotenvx/primitives for fixed keys.
//   node generate.js path/to/node_modules/@dotenvx/primitives
const fs = require('fs')
const path = require('path')
const VERSION = '4.3.0'
const pkg = require(path.resolve(process.argv[2], 'package.json'))
if (pkg.name !== '@dotenvx/primitives' || pkg.version !== VERSION) throw new Error(`want ${VERSION}`)
const p = require(path.resolve(process.argv[2]))
const privateKey = 'ec9e80073d7ace817d35acb8b7293cbf8e5981b4d2f5708ee5be405122993cd1'
const plaintexts = ['', 'a', 'hello world', 'héllo wörld ✓ 𝄞', 'x'.repeat(1000), 'line1\nline2']
const vectors = plaintexts.map(plaintext => ({ plaintext, encrypted: p.encrypt(p.derive(privateKey), plaintext) }))
for (const v of vectors) if (p.decrypt(privateKey, v.encrypted) !== v.plaintext) throw new Error('round trip')
fs.writeFileSync(path.join(__dirname, 'vectors.json'), JSON.stringify({ privateKey, publicKey: p.derive(privateKey), vectors }, null, 2) + '\n')
