import {registry} from './model.js';
export const systemSections={principle:'Foundations',rule:'Rules & mechanics',capability:'Capabilities',method:'Methods & procedures',resource:'Resources',component:'Components',implementation:'Devices & implementations',application:'Applications & consequences',development:'Discoveries & history'};
export const templates={
 'Rule-based magic':['What powers the system?','How is power accessed?','Which limits are absolute?'],
 'Divine or supernatural powers':['Where does power originate?','Who can access it?','What remains unknowable?'],
 'Elemental magic':['What domains exist?','How do they interact?','What does an effect cost?'],
 'Ritual magic':['What conditions enable a ritual?','What materials or participants are needed?','What can fail?'],
 'Craft and engineering':['Which principles are understood?','What materials and skills are needed?','How are tools maintained?'],
 'Industrial technology':['Where does energy come from?','What constrains production?','What infrastructure is available?'],
 'Advanced science':['What is possible?','What is only theoretical?','Which limits remain?'],
 'Enchanted technology':['How do mechanisms combine?','What magical and physical inputs are required?','Who can reproduce it?'],
 'Alchemy':['What transforms?','Which catalysts are needed?','What is conserved or consumed?'],
 'Custom system':['What does this system allow?','What does it require?','What remains unresolved?'],
};
export const isSystem=e=>registry.types[e?.kind]?.category==='systems';
export const systemRelations=Object.keys(registry.relations).filter(k=>k.startsWith('system-'));
export function compatible(kind,source,destination){const r=registry.relations[kind],matches=(e,allowed)=>e&&(!allowed?.length||allowed.includes(e.kind)||allowed.includes(registry.types[e.kind]?.category));return source?.id!==destination?.id&&matches(source,r?.sources)&&matches(destination,r?.destinations)}
export function recordCommand(age,entity,original,relations){
 const ids=new Set(relations.map(r=>r.id));
 const removed=original.filter(r=>!ids.has(r.id)).map(r=>r.id);
 const changed=relations.filter(r=>JSON.stringify(r)!==JSON.stringify(original.find(x=>x.id===r.id)));
 const commands=[{kind:'entity',entity}];
 if(changed.length||removed.length)commands.push({kind:'relations',relations:changed,removeRelationIds:removed});
 return {kind:'batch',ageId:age,commands};
}
export const issueKey=i=>JSON.stringify([i.entityId,i.code,i.message,i.related]);
export function issueChanges(before=[],after=[]){const a=new Set(before.map(issueKey)),b=new Set(after.map(issueKey));return {added:after.filter(i=>!a.has(issueKey(i))),resolved:before.filter(i=>!b.has(issueKey(i)))}}
export function newRecord(name,kind,systemId,category='magic'){return {id:crypto.randomUUID(),name,kind,properties:{},system:{version:1,...(kind==='system'?{category,mode:'quick'}:{systemId}),possibility:'unknown'}}}
