import test from 'node:test';
import assert from 'node:assert/strict';
import { unzipSync, strFromU8 } from 'fflate';
import { orderedNodes, manuscriptNodes, words, plainDocument, textContent, continuity, inlineReferences } from '../src/worldbuilding/writer/model.js';
import { markdown } from '../src/worldbuilding/writer/editor.js';
import { exportDocx, exportText } from '../src/worldbuilding/writer/export.js';
const node=(id,parentId='',position=0,type='scene')=>({id,parentId,position,type,title:id,content:plainDocument('A scene.'),references:[]});
test('canonical manuscript order is recursive; private subtrees excluded unless explicitly selected',()=>{
 const nodes=[node('second','chapter',2),node('chapter','book',0,'chapter'),node('first','chapter',1),node('book','',0,'book'),node('private','',1,'research'),node('private child','private',0)];
 assert.deepEqual(orderedNodes(nodes).map(n=>n.id),['book','chapter','first','second','private','private child']);
 assert.deepEqual(manuscriptNodes(nodes,'chapter').map(n=>n.id),['chapter','first','second']);
 assert.equal(manuscriptNodes(nodes).length,4);assert.equal(manuscriptNodes(nodes,'private').length,0);assert.equal(manuscriptNodes(nodes,'private',true).length,2);
 nodes.find(n=>n.id==='second').position=0;assert.deepEqual(manuscriptNodes(nodes,'chapter').map(n=>n.id),['chapter','second','first']);
});
test('semantic Markdown round-trip, scene breaks, Unicode counts and world wording survive export',()=>{
 const source='# Opening\n\n**Bold** and *italic*.\n\n> A memory\n\n- First\n- Second\n\n---\n\n[Safe link](https://example.com)';
 const content=markdown.parse(source),rendered=markdown.serialize(content);assert.match(rendered,/\*\*Bold\*\*/);assert.match(rendered,/\*italic\*/);assert.match(rendered,/> A memory/);assert.ok(content.content.some(n=>n.type==='horizontalRule'));
 const ref={type:'text',text:'Original wording',marks:[{type:'worldReference',attrs:{worldId:'w',ageId:'a',entityId:'e'}}]};content.content.push({type:'paragraph',content:[ref]});assert.match(markdown.serialize(content),/Original wording/);assert.equal(inlineReferences(content)[0].entityId,'e');
 assert.equal(words('Hello, світ! A well-known river.\n***'),5);
 assert.match(exportText([{title:'Chapter',content}],'txt'),/\*\*\*/);
 const zip=unzipSync(exportDocx([{title:'A & B',content}]));const xml=strFromU8(zip['word/document.xml']);assert.match(xml,/A &amp; B/);assert.match(xml,/<w:b\/>/);assert.match(xml,/Original wording/);
});
test('continuity checks selected historical state and preserves reference identity after rename',()=>{
 const n={...node('s'),storyAgeId:'old',storyYear:5,references:[{worldId:'w',ageId:'old',entityId:'hero',wording:'Old name'}]};
 const doc={entities:[{id:'hero',ageId:'old',name:'New name',kind:'character',properties:{startYear:10}},{id:'hero',ageId:'new',name:'New name',kind:'character',properties:{startYear:1}}]};
 const issues=continuity(n,doc,'w');assert.equal(issues[0].name,'New name');assert.match(issues[0].reason,/precedes/);n.storyYear=15;assert.equal(continuity(n,doc,'w').length,0);n.storyAgeId='missing';assert.match(continuity(n,doc,'w')[0].reason,/missing/);
});
