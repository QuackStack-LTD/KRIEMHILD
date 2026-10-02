import fs from 'node:fs';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
const here=path.dirname(fileURLToPath(import.meta.url));
const root=path.resolve(here,'../../..');
const rows=JSON.parse(fs.readFileSync(path.join(here,'repository-inventory.json'),'utf8')).repositories;
const start=Number(process.argv[2]||0), count=Number(process.argv[3]||10);
for(const r of rows.slice(start,start+count)) {
 const readme=r.readmes.map(f=>fs.readFileSync(path.join(root,r.name,f),'utf8')).join('\n');
 console.log(`\n## ${r.name} | ${r.remote} | ${r.commit}\nLICENSE: ${r.licenses.map(l=>`${l.path}: ${l.header.slice(0,180).replace(/\s+/g,' ')}`).join('; ')||'No root license file found'}\nREADME:\n${readme.slice(0,2000)}\nCANDIDATES: ${r.sourceCandidates.join(', ')}`);
 for(const f of r.sourceCandidates.slice(0,2)) {
  console.log(`\nSOURCE ${f}:\n${fs.readFileSync(path.join(root,r.name,f),'utf8').slice(0,1100)}`);
 }
 if(!r.sourceCandidates.length) console.log(`ROOT ENTRIES: ${fs.readdirSync(path.join(root,r.name)).filter(f=>f!=='.git').slice(0,25).join(', ')}`);
}
