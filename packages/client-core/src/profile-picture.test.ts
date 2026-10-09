import test from 'node:test';
import assert from 'node:assert/strict';
import { createPictureOperationOwner, type PictureChange } from './profile-picture';

test('picture retry freezes its crop and key; account replacement discards completion', async () => {
 let sequence=0;
 const operation=createPictureOperationOwner(()=>`picture-key-${++sequence}`);
 const crop={x:0,y:0,size:16};
 const change:PictureChange={userId:'owner',revision:1,image:'aGVsbG8=',crop,remove:false};
 const keys:string[]=[];
 const first=await operation.submit(change,async(value,key)=>{keys.push(key);assert.notEqual(value.crop,crop);throw new Error('lost response');});
 assert.equal(first.kind,'failed');
 let finish!: (value:{userId:string;url:string;revision:number})=>void;
 const retry=operation.submit(change,(_,key)=>{keys.push(key);return new Promise(resolve=>{finish=resolve;});});
 operation.cancel();
 finish({userId:'owner',url:'https://api.example.test/picture',revision:2});
 assert.equal((await retry).kind,'superseded');assert.equal(keys[0],keys[1]);
 const fresh=await operation.submit({...change,userId:'other'},async(value,key)=>{assert.notEqual(key,keys[0]);return {userId:value.userId,url:'https://api.example.test/new',revision:2};});
 assert.equal(fresh.kind,'applied');
});

test('removal cannot include image data and preview cancellation drops delayed image bytes',async()=>{
 const operation=createPictureOperationOwner(()=> 'operation-key');
 let writes=0;
 const result=await operation.submit({userId:'owner',revision:1,image:'aGVsbG8=',remove:true},async()=>{writes++;return {userId:'owner',url:'',revision:2};});
 assert.equal(result.kind,'failed');assert.equal(writes,0);
 let finish!: (value:{image:string;width:number;height:number})=>void;
 const preview=operation.preview('aGVsbG8=',()=>new Promise(resolve=>{finish=resolve;}));
 operation.cancel();finish({image:'/9j/2Q==',width:16,height:16});
 assert.equal((await preview).kind,'superseded');
});
