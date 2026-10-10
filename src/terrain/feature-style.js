// Shared by map textures, flat maps, and the Builder's 3D object overlay.
// Styling depends on feature type, never on whether it was authored or generated.
export const riverStyle=Object.freeze({water:'#397f9e',bank:'#a1a183',highlight:'#72b6c5',surface:'#397f9e'});
export const entityStyle=Object.freeze({color:'#000000',selected:'#ffcc66',lineWidth:2,pointSize:10});
export function entityColor(entity,selected=false){return selected?entityStyle.selected:entity.color||entityStyle.color;}
