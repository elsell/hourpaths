import { PictureFailure, validatePictureBytes } from '@hourpaths/client-core';

/** Reads only the file explicitly selected by the user; never persists its data. */
export function readBrowserPictureFile(file:File,signal?:AbortSignal):Promise<string>{
 if(file.size>10_000_000)return Promise.reject(new PictureFailure('too_large'));
 if(!file.size)return Promise.reject(new PictureFailure('invalid'));
 return new Promise((resolve,reject)=>{
  const reader=new FileReader();
  const abort=()=>reader.abort();
  const cleanup=()=>signal?.removeEventListener('abort',abort);
  reader.onload=()=>{cleanup();try{if(typeof reader.result!=='string')throw new PictureFailure();const comma=reader.result.indexOf(',');if(comma<0)throw new PictureFailure();resolve(validatePictureBytes(reader.result.slice(comma+1)));}catch(error){reject(error);}};
  reader.onerror=()=>{cleanup();reject(new PictureFailure());};
  reader.onabort=()=>{cleanup();reject(new PictureFailure('rejected'));};
  if(signal?.aborted){reject(new PictureFailure('rejected'));return;}
  signal?.addEventListener('abort',abort,{once:true});reader.readAsDataURL(file);
 });
}
