// Read-only inventory of sibling reference projects. Run from any directory.
import fs from 'node:fs';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
import {execFileSync} from 'node:child_process';
const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '../../..');
const git = (dir, args) => { try { return execFileSync('git', ['-C', dir, ...args], {encoding:'utf8', stdio:['ignore','pipe','ignore']}).trim(); } catch { return null; } };
function walk(dir, prefix='') {
  return fs.readdirSync(dir, {withFileTypes:true}).flatMap(e => {
    if (['.git','node_modules','.next','vendor','dist','build'].includes(e.name)) return [];
    const rel = prefix ? `${prefix}/${e.name}` : e.name;
    return e.isDirectory() ? walk(path.join(dir,e.name),rel) : [rel];
  });
}
const rows = fs.readdirSync(root,{withFileTypes:true}).filter(e=>e.isDirectory() && e.name!=='KRIEMHILD').sort((a,b)=>a.name.localeCompare(b.name)).map(e=>{
  const dir=path.join(root,e.name);
  const tracked=git(dir,['ls-files']);
  const files=tracked ? tracked.split('\n') : walk(dir);
  const readmes=files.filter(f=>/^readme[^/]*$/i.test(f));
  const licenses=files.filter(f=>/^(license|licence|copying|license-cc-by)[^/]*$/i.test(f));
  const manifests=files.filter(f=>/(^|\/)(package.json|go.mod|Cargo.toml|pyproject.toml|composer.json|pom.xml|Project.toml|DESCRIPTION|dune-project)$/.test(f));
  const text=f=>{try{return fs.readFileSync(path.join(dir,f),'utf8');}catch{return '';}};
  const source=files.filter(f=>/\.(go|ts|tsx|js|jsx|py|lua|jl|php|cs|java|gd|clj|cljs|rs|ml|R|xqm|xql|html)$/.test(f) && !/(test|spec|vendor|externals|\.min\.|lock|generated|wailsjs|fixture|node_modules)/i.test(f));
  const score=f=>(/(model|schema|entity|terrain|history|family|lexicon|world|simulation|store|project|document|graph|calendar|map|econom|rule|character)/i.test(f)?20:0)+(f.split('/').length<4?5:0)-(f.length/100);
  source.sort((a,b)=>score(b)-score(a));
  return {name:e.name, remote:git(dir,['remote','get-url','origin']), commit:git(dir,['rev-parse','HEAD']), fileCount:files.length, inventoryMethod:tracked?'git ls-files':'filesystem walk', readmes, licenses:licenses.map(f=>({path:f,header:text(f).slice(0,200)})), manifests, sourceCandidates:source.slice(0,18)};
});
fs.writeFileSync(path.join(here,'repository-inventory.json'),JSON.stringify({scope:'Local checkout evidence; not a runtime or security audit',repositories:rows},null,2)+'\n');
console.log(`Inventoried ${rows.length} reference directories.`);
