import {createContext,useContext} from 'react';
export const WorldProjectContext=createContext(null);
export const useWorldProject=()=>useContext(WorldProjectContext);
export async function worldRequest(id, suffix='', body){
 const r=await fetch(`/api/sessions/${id}/world${suffix}`,{method:body===undefined?'GET':'POST',headers:{'Content-Type':'application/json'},body:body===undefined?undefined:JSON.stringify(body)});
 const data=await r.json();if(!r.ok){const e=new Error(data.error||'World request failed');e.status=r.status;e.project=data.project;throw e;}return data;
}
export function parseRoute(path){if(path==='/')return {view:'home'};if(path==='/world/new')return {view:'generate',fresh:true};const m=path.match(/^\/world\/([a-f0-9]{32})\/(generate|build)\/?$/);return m?{id:m[1],view:m[2]}:{view:'missing'};}
