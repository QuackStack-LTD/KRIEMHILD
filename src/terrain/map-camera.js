export const DETAIL_NAMES = ['World', 'Continental', 'Regional', 'Local', 'Maximum detail'];
export const smooth = x => { x=Math.max(0,Math.min(1,x));return x*x*(3-2*x); };
export function detailAtScale(pixelsPerCell) { return Math.max(0,Math.min(8,Math.log2(pixelsPerCell/6))); }
export function detailName(level) { return DETAIL_NAMES[Math.min(4,Math.floor((level+1)/2))]; }
export class MapCamera {
  constructor(){this.scale=1;this.x=0;this.y=0;this.minimum=1;}
  fit(width,height,worldWidth,worldHeight){this.minimum=Math.min(width/worldWidth,height/worldHeight);this.scale=this.minimum;this.x=(width-worldWidth*this.scale)/2;this.y=(height-worldHeight*this.scale)/2;}
  point(x,y){return {x:(x-this.x)/this.scale,y:(y-this.y)/this.scale};}
  zoomAt(x,y,factor){const p=this.point(x,y);this.scale=Math.max(this.minimum,Math.min(Math.max(this.minimum,3072),this.scale*factor));this.x=x-p.x*this.scale;this.y=y-p.y*this.scale;}
  bounds(width,height){const a=this.point(0,0),b=this.point(width,height);return {x:a.x,y:a.y,width:b.x-a.x,height:b.y-a.y};}
}
