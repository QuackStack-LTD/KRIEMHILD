import {spawnSync,spawn} from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'..');
const windows=process.platform==='win32';
const localGo=path.join(root,'.tools','go','bin',windows?'go.exe':'go');
const go=process.env.KRIEMHILD_GO || (fs.existsSync(localGo)?localGo:'go');
const npm=windows?'npm.cmd':'npm';
const npmCLI=process.env.npm_execpath || path.join(path.dirname(process.execPath),'node_modules','npm','bin','npm-cli.js');
const binary=path.join(root,'bin',windows?'kriemhild-dev.exe':'kriemhild-dev');
function run(command,args,cwd=root){
 const isNpm=command===npm;
 const result=spawnSync(isNpm?process.execPath:command,isNpm?[npmCLI,...args]:args,{cwd,stdio:'inherit',env:{...process.env,NEXT_TELEMETRY_DISABLED:'1'}});
 if(result.error){console.error(result.error.message);process.exit(1);}
 if(result.status!==0)process.exit(result.status||1);
}
const web=path.join(root,'apps','web');
switch(process.argv[2]){
 case 'demo':run(go,['run','./cmd/kriemhild-demo',...process.argv.slice(3)]);break;
 case 'setup':run(go,['version']);run(npm,['ci','--no-fund'],web);break;
 case 'build':run(npm,['run','build'],web);fs.mkdirSync(path.dirname(binary),{recursive:true});run(go,['build','-o',binary,'./cmd/kriemhild']);break;
 case 'test':run(go,['test','./...']);run(go,['vet','./...']);run(npm,['run','typecheck'],web);break;
 case 'format':run(go,['fmt','./...']);run(npm,['exec','--','prettier','--write','app','components','lib','tests','next.config.ts','playwright.config.ts'],web);break;
 case 'e2e':run(npm,['run','test:e2e'],web);break;
 case 'start':{
  if(!fs.existsSync(binary)){console.error('Run npm run build first.');process.exit(1);}
  const child=spawn(binary,process.argv.slice(3),{cwd:root,stdio:'inherit'});
  child.on('error',e=>{console.error(e.message);process.exitCode=1;});
  child.on('exit',code=>{process.exitCode=code||0;});
  break;
 }
 default:console.error('Expected setup, build, start, test, e2e or format');process.exit(1);
}
