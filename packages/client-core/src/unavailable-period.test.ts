import assert from 'node:assert/strict';
import test from 'node:test';
import { createUnavailablePeriodOwner, type UnavailablePeriodPreference } from './unavailable-period';

test('quiet-hours retries preserve identity and cannot cross account lifetimes', async () => {
 let count=0;const keys:string[]=[];
 const owner=createUnavailablePeriodOwner('alice',()=>`key-${++count}`);
 const change={userId:'alice',enabled:true,startMinute:1320,endMinute:480,expectedRevision:0,reviewedTimeZone:'Etc/UTC'};
 const first=await owner.submit(change,async (_,key)=>{keys.push(key);throw Error('lost_response');});assert.equal(first.kind,'failed');
 const next=await owner.submit(change,async (value,key)=>{keys.push(key);return {userId:value.userId,enabled:value.enabled,startMinute:value.startMinute,endMinute:value.endMinute,revision:1,timeZone:value.reviewedTimeZone};});
 assert.equal(next.kind,'applied');assert.equal(keys[0],keys[1]);
 let finish!:(value:UnavailablePeriodPreference)=>void;
 const pending=owner.submit({...change,expectedRevision:1},()=>new Promise(resolve=>{finish=resolve;}));
 assert.equal((await owner.submit(change,async()=>{throw Error('must not run');})).kind,'busy');
 owner.cancel();finish({userId:'alice',enabled:true,startMinute:1320,endMinute:480,revision:2,timeZone:'Etc/UTC'});assert.equal((await pending).kind,'superseded');
 let wrongAccountReached=false;
 assert.equal((await owner.submit({...change,userId:'bob'},async()=>{wrongAccountReached=true;return {userId:'bob',enabled:true,startMinute:1320,endMinute:480,revision:1,timeZone:'Etc/UTC'};})).kind,'failed');
 assert.equal(wrongAccountReached,false);
});
