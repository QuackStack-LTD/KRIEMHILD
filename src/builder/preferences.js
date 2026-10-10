import {entityStyle} from '../terrain/feature-style.js';
import {allowedGeometry} from './geometry.js';
const defaults={tool:'select',layer:'settlements',kind:'city',name:'New city',color:entityStyle.color,radius:2,amount:100,overlay:'terrain'};
export function builderPreferences(view){
 const p={...defaults},saved=view?.tools;
 if(!saved||typeof saved!=='object')return p;
 const choices={tool:['select','pan','point','line','polygon','raise','lower','flatten','smooth','crater','valley','island','water','drain','river','plant','clear'],layer:['settlements','roads','buildings','countries','borders','labels'],overlay:['terrain','elevation','waterDepth','watershed','accumulation','riverClass','riverSystem']};
 for(const key in choices)if(choices[key].includes(saved[key]))p[key]=saved[key];
 for(const key of ['kind','name'])if(typeof saved[key]==='string'&&saved[key].length<=160)p[key]=saved[key];
 if(/^#[0-9a-f]{6}$/i.test(saved.color||''))p.color=saved.color;
 if(Number.isFinite(+saved.radius))p.radius=Math.max(.01,Math.min(255,+saved.radius));
 if(Number.isFinite(+saved.amount))p.amount=Math.max(-12000,Math.min(12000,+saved.amount));
 if(['point','line','polygon'].includes(p.tool)&&!allowedGeometry(p.layer,p.kind).includes(p.tool))p.tool=allowedGeometry(p.layer,p.kind)[0];
 return p;
}
