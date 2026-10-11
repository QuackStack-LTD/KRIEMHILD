import {getRiverThickness,normalizeRiverThickness} from './river-display.js';
// Physical widths come from the saved geological channel model. Optical
// scaling makes subpixel channels legible without collapsing their hierarchy.
export function riverWidthPixels(feature,index,pixelsPerCell,detail,thickness=getRiverThickness()){
  const local=Math.max(0,feature.widths?.[index]??feature.width??0);
  const physical=local*pixelsPerCell;
  const q=Math.max(0,feature.dischargesM3s?.[index]??feature.dischargeM3s??feature.discharge??0);
  const reference=feature.physicalWidthMetres||0;
  const metres=reference>0?reference*(feature.width>0?local/feature.width:1):3.6*Math.pow(q,.56);
  const zoom=Math.max(0,Math.min(8,detail));
  // Keep overview strokes restrained; closer views progressively amplify
  // differences around a narrow 25 m channel instead of thickening everything.
  const contrast=.13*Math.min(1,zoom/6);
  const symbol=metres>0?.065*Math.pow(metres,.45)*Math.pow(metres/25,contrast):.12;
  return Math.max(physical,symbol*2**(zoom*.48))*normalizeRiverThickness(thickness);
}
