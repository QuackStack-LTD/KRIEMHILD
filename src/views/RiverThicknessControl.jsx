import {useId,useSyncExternalStore} from 'react';
import {getRiverThickness,setRiverThickness,subscribeRiverThickness} from '../terrain/river-display.js';
import './river-thickness.css';

export default function RiverThicknessControl(){
  const id=useId(),value=useSyncExternalStore(subscribeRiverThickness,getRiverThickness);
  return <section className="river-thickness-control">
    <label htmlFor={id}>River display thickness <output htmlFor={id}>{value.toFixed(2)}x</output></label>
    <div><input id={id} type="range" min="0.25" max="4" step="0.05" value={value} aria-describedby={`${id}-help`} onChange={e=>setRiverThickness(e.target.value)}/><button type="button" disabled={value===1} onClick={()=>setRiverThickness(1)}>Reset</button></div>
    <small id={`${id}-help`}>Display only. Preserves relative river sizes and physical riverbeds. Remembered in this browser.</small>
  </section>;
}
