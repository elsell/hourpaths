import { createSessionApiClient } from '@hourpaths/api-client';
import { PictureFailure, freezePictureChange, validatePictureBytes, validatePicturePreview, type ProfilePicture, type ProfilePictureRepository } from '../profile-picture';

function picture(value:unknown,owner:string,baseURL:string):ProfilePicture {
 if(!value||typeof value!=='object')throw new PictureFailure();
 const row=value as Partial<ProfilePicture>;
 if(row.userId!==owner||typeof row.url!=='string'||!Number.isSafeInteger(row.revision)||row.revision!<1)throw new PictureFailure();
 let location=row.url;
 if(location){
  let url:URL;try{url=new URL(location);}catch{throw new PictureFailure();}
  if(url.username||url.password)throw new PictureFailure();
  // Application media identifiers resolve against the already validated API
  // origin, including emulator loopback in explicit development environments.
  if(/^\/v1\/profile-pictures\/[0-9a-f-]{36}$/.test(url.pathname)&&!url.search&&!url.hash){location=new URL(url.pathname,baseURL).href;}
  else if(url.protocol!=='https:')throw new PictureFailure();
 }
 return {userId:owner,url:location,revision:row.revision!};
}
function failure(status:number):PictureFailure{return new PictureFailure(status===409?'conflict':status===413?'too_large':status===400||status===422?'invalid':status===401||status===403?'rejected':status===429?'rate_limited':'unavailable');}
/** A picture operation binds the credential and owner before any upload begins. */
export function apiProfilePicture(baseURL:string,token:string|null,owner:string,rejected?:(token:string|null)=>void,signal?:AbortSignal):ProfilePictureRepository {
 const api=createSessionApiClient(baseURL,()=>token,signal,rejected);
 return {
  async read(){const result=await api.ownProfilePicture();if(!result.response.ok||!result.data)throw failure(result.response.status);return picture(result.data.data,owner,baseURL);},
  async preview(image){const result=await api.previewProfilePicture(validatePictureBytes(image));if(!result.response.ok||!result.data)throw failure(result.response.status);return validatePicturePreview(result.data.data);},
  async save(value,key){const frozen=freezePictureChange(value);if(frozen.userId!==owner)throw new PictureFailure('rejected');const result=await api.updateProfilePicture({image:frozen.image,crop:frozen.crop,remove:frozen.remove,expectedRevision:frozen.revision},key);if(!result.response.ok||!result.data)throw failure(result.response.status);return picture(result.data.data,owner,baseURL);},
 };
}
