import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
const registry=await readFile(new URL('../internal/history/registry.json',import.meta.url),'utf8');
const source=(await readFile(new URL('../src/worldbuilding/model.js',import.meta.url),'utf8')).replace("import registry from '../../internal/history/registry.json';",`const registry=${registry};`);
const modelURL='data:text/javascript;base64,'+Buffer.from(source).toString('base64');
const code=(await readFile(new URL('../src/worldbuilding/systems.js',import.meta.url),'utf8')).replace("'./model.js'",JSON.stringify(modelURL));
const {compatible,issueChanges,recordCommand,templates}=await import('data:text/javascript;base64,'+Buffer.from(code).toString('base64'));
test('system relationships separate knowledge, supply and mechanical dependencies',()=>{
 const person={id:'p',kind:'character'},rule={id:'r',kind:'system-rule'},city={id:'c',kind:'city'};
 assert.ok(compatible('system-knowledge',person,rule));
 assert.ok(!compatible('system-knowledge',rule,person));
 assert.ok(!compatible('system-requires',city,rule));
 assert.ok(!compatible('system-requires',rule,rule));
});
test('draft commits an atomic delta without removing unrelated relations',()=>{
 const one={id:'1',kind:'system-requires'},two={id:'2',kind:'system-consumes'},three={id:'3',kind:'system-requires'};
 const command=recordCommand('age',{id:'record'},[one,two],[{...one,description:'changed'},three]);
 assert.equal(command.kind,'batch');assert.deepEqual(command.commands[1].removeRelationIds,['2']);assert.equal(command.commands[1].relations.length,2);
 assert.equal(recordCommand('age',{id:'record'},[one],[one]).commands.length,1);
});
test('impact review distinguishes new and resolved issues and templates only ask questions',()=>{
 const old={entityId:'e',code:'x',message:'Old'},now={entityId:'e',code:'y',message:'New'};
 assert.deepEqual(issueChanges([old],[now]),{added:[now],resolved:[old]});
 assert.ok(Object.values(templates).every(q=>q.every(x=>x.endsWith('?'))));
});
