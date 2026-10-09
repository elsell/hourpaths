import { useMemo, useRef } from 'react';
import { validatePicturePreview, type PicturePreview, type PictureCrop } from '@hourpaths/client-core';
import type { Translator } from '@hourpaths/i18n';

export function PictureCropEditor({preview,crop,onChange,disabled,i18n}:{preview:PicturePreview;crop:PictureCrop;onChange(value:PictureCrop):void;disabled:boolean;i18n:Translator}){
 const source=useMemo(()=>`data:image/jpeg;base64,${validatePicturePreview(preview).image}`,[preview]);
 const held=useRef<{x:number;y:number;crop:PictureCrop}|null>(null);
 const clamp=(value:number,maximum:number)=>Math.max(0,Math.min(maximum,Math.round(value)));
 function zoom(value:number){const size=Math.max(1,Math.round(Math.min(preview.width,preview.height)/value));onChange({size,x:clamp(crop.x+(crop.size-size)/2,preview.width-size),y:clamp(crop.y+(crop.size-size)/2,preview.height-size)});}
 return <div className="studio-picture-crop">
  <div className="studio-picture-viewport" onPointerDown={event=>{if(disabled)return;event.currentTarget.setPointerCapture(event.pointerId);held.current={x:event.clientX,y:event.clientY,crop};}} onPointerUp={()=>{held.current=null;}} onPointerCancel={()=>{held.current=null;}} onPointerMove={event=>{const start=held.current;if(!start||disabled)return;const scale=start.crop.size/event.currentTarget.clientWidth;onChange({...start.crop,x:clamp(start.crop.x-(event.clientX-start.x)*scale,preview.width-start.crop.size),y:clamp(start.crop.y-(event.clientY-start.y)*scale,preview.height-start.crop.size)});}}>
   <img src={source} alt={i18n.t('picture.preview')} draggable={false} referrerPolicy="no-referrer" style={{width:`${preview.width/crop.size*100}%`,height:`${preview.height/crop.size*100}%`,left:`${-crop.x/crop.size*100}%`,top:`${-crop.y/crop.size*100}%`}} />
  </div>
  <p>{i18n.t('picture.position')}</p>
  <label>{i18n.t('picture.zoom')}<input type="range" min="1" max="4" step="0.01" disabled={disabled} value={Math.min(preview.width,preview.height)/crop.size} onChange={event=>zoom(Number(event.target.value))}/></label>
  <label>{i18n.t('picture.horizontal')}<input type="range" min="0" max={preview.width-crop.size} step="1" disabled={disabled||preview.width===crop.size} value={crop.x} onChange={event=>onChange({...crop,x:Number(event.target.value)})}/></label>
  <label>{i18n.t('picture.vertical')}<input type="range" min="0" max={preview.height-crop.size} step="1" disabled={disabled||preview.height===crop.size} value={crop.y} onChange={event=>onChange({...crop,y:Number(event.target.value)})}/></label>
 </div>;
}
