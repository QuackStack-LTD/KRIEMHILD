import fs from 'node:fs';
import path from 'node:path';
import {spawnSync} from 'node:child_process';
import {fileURLToPath} from 'node:url';
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'..');
const lock=JSON.parse(fs.readFileSync(path.join(root,'apps/web/package-lock.json'),'utf8'));
const notices=['# Dependency notices\n\nGenerated from the locked packages and installed license files. Includes build dependencies; no source code was copied from the 80 reference repositories for these implementation slices.\n'];
const components=[];
for(const [key,info] of Object.entries(lock.packages)){
 if(!key)continue;
 const folder=path.join(root,'apps/web',key);
 let pkg;try{pkg=JSON.parse(fs.readFileSync(path.join(folder,'package.json'),'utf8'))}catch{continue}
 components.push({type:'npm',name:pkg.name,version:info.version,license:pkg.license||'See package license',integrity:info.integrity,development:!!info.dev});
 const files=fs.readdirSync(folder).filter(f=>/^licen[cs]e(?:[.-]|$)|^copying(?:[.-]|$)|^notice(?:[.-]|$)/i.test(f));
 notices.push(`\n## ${pkg.name} ${info.version}\n\nLicense: ${typeof pkg.license==='string'?pkg.license:JSON.stringify(pkg.license||'See files')}\n`);
 for(const file of files){const filename=path.join(folder,file);if(fs.statSync(filename).isFile())notices.push(`\n${file}:\n\n${fs.readFileSync(filename,'utf8')}\n`)}
}
const local=path.join(root,'.tools/go/bin',process.platform==='win32'?'go.exe':'go');
const go=process.env.KRIEMHILD_GO||(fs.existsSync(local)?local:'go');
const result=spawnSync(go,['list','-m','-json','all'],{cwd:root,encoding:'utf8'});
if(result.status!==0)throw new Error('Cannot enumerate Go modules');
const objects=result.stdout.trim().split(/\r?\n}\r?\n(?={)/).map((s,i,a)=>JSON.parse(i<a.length-1?s+'\n}':s));
for(const module of objects){if(module.Main)continue;components.push({type:'go',name:module.Path,version:module.Version});notices.push(`\n## ${module.Path} ${module.Version}\n`);if(module.Dir){for(const file of fs.readdirSync(module.Dir).filter(f=>/^licen[cs]e|^copying|^notice/i.test(f))){const filename=path.join(module.Dir,file);if(fs.statSync(filename).isFile())notices.push(`\n${file}:\n\n${fs.readFileSync(filename,'utf8')}\n`)}}}
fs.writeFileSync(path.join(root,'docs/DEPENDENCY_NOTICES.md'),notices.join('\n'));
fs.writeFileSync(path.join(root,'docs/dependencies.json'),JSON.stringify({format:'kriemhild-dependency-inventory-v1',components},null,2)+'\n');
console.log(`Inventoried ${components.length} dependencies.`);
