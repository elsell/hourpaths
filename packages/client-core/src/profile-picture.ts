export interface ProfilePicture { userId: string; url: string; revision: number }
export interface PictureCrop { x: number; y: number; size: number }
export interface PicturePreview { image: string; width: number; height: number }
export interface PictureChange { userId: string; revision: number; image: string; crop?: PictureCrop; remove: boolean }
export interface ProfilePictureRepository {
 read(): Promise<ProfilePicture>;
 preview(image: string): Promise<PicturePreview>;
 save(value: PictureChange, key: string): Promise<ProfilePicture>;
}
export class PictureFailure extends Error {
 constructor(readonly kind: 'invalid'|'too_large'|'conflict'|'rejected'|'rate_limited'|'unavailable'='unavailable'){super(`profile_picture_${kind}`);}
}
export function validatePictureBytes(value: string, maxBytes=10_000_000): string {
 if(typeof value!=='string'||!value.length||value.length>Math.ceil(maxBytes/3)*4)throw new PictureFailure(value?.length?'too_large':'invalid');
 if(value.length%4!==0||!/^[A-Za-z0-9+/]+={0,2}$/.test(value))throw new PictureFailure('invalid');
 const size=value.length/4*3-(value.endsWith('==')?2:value.endsWith('=')?1:0);
 if(size>maxBytes)throw new PictureFailure('too_large');
 return value;
}
export function validatePicturePreview(value: PicturePreview): PicturePreview {
 validatePictureBytes(value.image,1_048_576);
 if(!value.image.startsWith('/9j/')||!Number.isSafeInteger(value.width)||!Number.isSafeInteger(value.height)||value.width<1||value.height<1||value.width>1024||value.height>1024)throw new PictureFailure();
 return {image:value.image,width:value.width,height:value.height};
}
export function centerPictureCrop(preview: Pick<PicturePreview,'width'|'height'>): PictureCrop {
 const size=Math.min(preview.width,preview.height);
 return {x:Math.floor((preview.width-size)/2),y:Math.floor((preview.height-size)/2),size};
}
export function freezePictureChange(value: PictureChange): PictureChange {
 if(!value.userId||!Number.isSafeInteger(value.revision)||value.revision<1||typeof value.remove!=='boolean')throw new PictureFailure('invalid');
 if(value.remove){if(value.image!==''||value.crop)throw new PictureFailure('invalid');return {userId:value.userId,revision:value.revision,image:'',remove:true};}
 validatePictureBytes(value.image);
 const crop=value.crop;
 if(!crop||![crop.x,crop.y,crop.size].every(Number.isSafeInteger)||crop.x<0||crop.y<0||crop.size<1||crop.size>1024||crop.x>1024-crop.size||crop.y>1024-crop.size)throw new PictureFailure('invalid');
 return {userId:value.userId,revision:value.revision,image:value.image,remove:false,crop:{...crop}};
}
function sameChange(a:PictureChange,b:PictureChange):boolean {
 return a.userId===b.userId&&a.revision===b.revision&&a.image===b.image&&a.remove===b.remove&&a.crop?.x===b.crop?.x&&a.crop?.y===b.crop?.y&&a.crop?.size===b.crop?.size;
}
export function createPictureOperationOwner(keyFactory:()=>string){
 let epoch=0,active:number|null=null;
 let retry:{value:PictureChange;key:string}|undefined;
 return {
  async preview(image:string,request:(image:string)=>Promise<PicturePreview>):Promise<{kind:'preview';preview:PicturePreview}|{kind:'failed';cause:unknown}|{kind:'busy'|'superseded'}>{
   if(active!==null)return {kind:'busy'};
   const generation=++epoch;active=generation;
   try{const preview=validatePicturePreview(await request(validatePictureBytes(image)));return epoch===generation?{kind:'preview',preview}:{kind:'superseded'};}
   catch(cause){return epoch===generation?{kind:'failed',cause}:{kind:'superseded'};}
   finally{if(active===generation)active=null;}
  },
  async submit(value:PictureChange,request:(value:PictureChange,key:string)=>Promise<ProfilePicture>):Promise<{kind:'applied';picture:ProfilePicture}|{kind:'failed';cause:unknown}|{kind:'busy'|'superseded'}>{
   if(active!==null)return {kind:'busy'};
   const generation=++epoch;active=generation;
   try{
    const frozen=freezePictureChange(value);
    const attempt=retry&&sameChange(retry.value,frozen)?retry:{value:frozen,key:keyFactory()};retry=attempt;
    const picture=await request(frozen,attempt.key);
    if(epoch!==generation)return {kind:'superseded'};
    if(picture.userId!==frozen.userId||picture.revision!==frozen.revision+1||typeof picture.url!=='string'||(frozen.remove?picture.url!=='':!picture.url))throw new PictureFailure();
    retry=undefined;return {kind:'applied',picture};
   }catch(cause){return epoch===generation?{kind:'failed',cause}:{kind:'superseded'};}
   finally{if(active===generation)active=null;}
  },
  cancel(){epoch++;active=null;retry=undefined;},
 };
}
