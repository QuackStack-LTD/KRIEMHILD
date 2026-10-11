// Browser-wide display preference, separate from persisted physical geography.
export const riverThicknessKey='kriemhild.riverThickness';
export function normalizeRiverThickness(value){
  const n=Number(value);
  return value===null||value===''||!Number.isFinite(n)?1:Math.max(.25,Math.min(4,n));
}
let thickness=1;
try{thickness=normalizeRiverThickness(globalThis.localStorage?.getItem(riverThicknessKey)??null);}catch{}
const listeners=new Set();
export function getRiverThickness(){return thickness;}
export function subscribeRiverThickness(listener,signal){
  if(signal?.aborted)return ()=>{};
  listeners.add(listener);
  const unsubscribe=()=>{listeners.delete(listener);signal?.removeEventListener('abort',unsubscribe);};
  signal?.addEventListener('abort',unsubscribe,{once:true});
  return unsubscribe;
}
export function setRiverThickness(value){
  const next=normalizeRiverThickness(value);
  if(next===thickness)return;
  thickness=next;
  try{globalThis.localStorage?.setItem(riverThicknessKey,String(next));}catch{}
  for(const listener of listeners)listener();
}
