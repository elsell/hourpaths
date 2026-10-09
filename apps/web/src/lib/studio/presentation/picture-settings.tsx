import { useEffect,useRef,useState } from 'react';
import { useQuery,useQueryClient } from '@tanstack/react-query';
import { createPictureOperationOwner,centerPictureCrop,PictureFailure,type ProfilePicture,type PicturePreview,type PictureCrop } from '@hourpaths/client-core';
import type { StudioDependencies } from './app';
import { useOwnedOperation } from './use-owned-operation';
import { UnsavedChanges } from './unsaved-changes';
import { PictureCropEditor } from './picture-crop';
import { Avatar } from './avatar';

export function PictureSettings({owner,dependencies:d}:{owner:string;dependencies:StudioDependencies}){
 const query=useQuery({queryKey:[d.accountScope,'preferences','picture'],queryFn:({signal})=>d.preferences.profilePicture(owner,signal)});
 return <section className="studio-settings-card"><h3>{d.i18n.t('picture.heading')}</h3><p>{d.i18n.t('picture.hint')}</p>
  {query.data?<PictureForm initial={query.data} dependencies={d}/>:query.isError?<div role="alert"><p>{d.i18n.t('picture.unavailable')}</p><button onClick={()=>void query.refetch()}>{d.i18n.t('common.retry')}</button></div>:<p role="status">{d.i18n.t('common.loading')}</p>}
 </section>;
}
function PictureForm({initial,dependencies:d}:{initial:ProfilePicture;dependencies:StudioDependencies}){
 const [saved,setSaved]=useState(initial),[image,setImage]=useState(''),[preview,setPreview]=useState<PicturePreview|null>(null),[crop,setCrop]=useState<PictureCrop|null>(null);
 const [busy,setBusy]=useState(false),[error,setError]=useState<PictureFailure|null>(null),[notice,setNotice]=useState<'saved'|'removed'|'review'|null>(null);
 const [owner]=useState(()=>createPictureOperationOwner(d.operationId));const operation=useOwnedOperation(d),cache=useQueryClient();
 useEffect(()=>()=>owner.cancel(),[owner]);
 const admission=useRef(false);
 const fail=(cause:unknown)=>setError(cause instanceof PictureFailure?cause:new PictureFailure());
 async function choose(file?:File){if(!file||admission.current||!d.pictureFiles)return;admission.current=true;setBusy(true);setError(null);setNotice(null);owner.cancel();try{
  const bytes=await operation.run(signal=>d.pictureFiles!.read(file,signal));if(!operation.active())return;
  const result=await owner.preview(bytes,image=>operation.run(signal=>d.preferences.previewPicture(saved.userId,image,signal)));if(!operation.active())return;
  if(result.kind==='preview'){setImage(bytes);setPreview(result.preview);setCrop(centerPictureCrop(result.preview));}else if(result.kind==='failed')fail(result.cause);
 }catch(cause){if(operation.active())fail(cause);}finally{admission.current=false;if(operation.active())setBusy(false);}}
 async function save(remove=false){if(admission.current||(!remove&&(!preview||!crop)))return;admission.current=true;setBusy(true);setError(null);setNotice(null);
  const result=await owner.submit({userId:saved.userId,revision:saved.revision,image:remove?'':image,crop:remove?undefined:crop!,remove},(value,key)=>operation.run(signal=>d.preferences.savePicture(value,key,signal)));
  admission.current=false;if(!operation.active())return;setBusy(false);
  if(result.kind==='applied'){setSaved(result.picture);setImage('');setPreview(null);setCrop(null);setNotice(remove?'removed':'saved');cache.setQueryData([d.accountScope,'preferences','picture'],result.picture);void cache.invalidateQueries({queryKey:[d.accountScope,'social']});}
  else if(result.kind==='failed')fail(result.cause);
 }
 async function reload(){if(admission.current)return;admission.current=true;setBusy(true);try{const value=await operation.run(signal=>d.preferences.profilePicture(saved.userId,signal));if(operation.active()){owner.cancel();setSaved(value);setError(null);setNotice('review');}}catch(cause){if(operation.active())fail(cause);}finally{admission.current=false;if(operation.active())setBusy(false);}}
 return <div className="studio-settings-form"><UnsavedChanges dirty={!!preview||busy} i18n={d.i18n}/>
  {preview&&crop?<PictureCropEditor preview={preview} crop={crop} onChange={value=>{setCrop(value);setNotice(null);}} disabled={busy} i18n={d.i18n}/>:<div className="studio-picture-current"><Avatar person={{id:saved.userId,name:'',username:'',picture:saved.url||null}}/></div>}
  <label>{d.i18n.t(saved.url?'picture.change':'picture.choose')}<input type="file" accept="image/jpeg,image/png,image/webp,image/heic,image/heif,.heic,.heif" disabled={busy||!d.pictureFiles} onChange={event=>{const file=event.currentTarget.files?.[0];event.currentTarget.value='';void choose(file);}}/></label>
  <div className="studio-settings-actions">{preview?<><button disabled={busy} onClick={()=>{owner.cancel();setImage('');setPreview(null);setCrop(null);setError(null);setNotice(null);}}>{d.i18n.t('common.cancel')}</button><button className="studio-primary" disabled={busy||error?.kind==='conflict'} onClick={()=>void save()}>{d.i18n.t('common.save')}</button></>:saved.url?<button disabled={busy} onClick={()=>void save(true)}>{d.i18n.t('picture.remove')}</button>:null}</div>
  {busy&&<p role="status">{d.i18n.t('common.loading')}</p>}{error&&<p role="alert">{d.i18n.t(`picture.${error.kind}`)}</p>}{error?.kind==='conflict'&&<button disabled={busy} onClick={()=>void reload()}>{d.i18n.t('picture.reload')}</button>}{notice&&<p role="status">{d.i18n.t(`picture.${notice}`)}</p>}
 </div>;
}
