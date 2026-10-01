import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { once } from 'node:events';
import { createLeaveRecovery, sharedLeaveCommands } from './studio/paths/adapters/shared-leave-commands';
import { createSessionApiClient } from '@hourpaths/api-client';
test('leave binds fresh capabilities and preserves destructive intent after a lost receipt', async () => {
  let allowed = true, wrongReceipt = false, archived = false; const writes: { key: string; body: unknown }[] = [];
  const server = createServer(async (request, response) => {
    response.setHeader('Content-Type', 'application/json');let data: unknown;
    if (request.method === 'DELETE') {
      let body='';for await(const part of request)body+=part;const parsed=JSON.parse(body);writes.push({key:String(request.headers['idempotency-key']),body:parsed});response.statusCode=writes.length===1?503:200;data={pathId:wrongReceipt?'other':'path',left:true,activityRetained:parsed.retainActivity};
    } else data={id:'path',name:'practice',archivedAt:archived?'2026-10-01T10:00:00Z':undefined,capabilities:{leavePath:allowed,trackTime:true}};
    response.end(JSON.stringify({data}));
  });server.listen(0,'127.0.0.1');await once(server,'listening');
  try {
    const port = (server.address() as {port:number}).port;
    const apiURL = `http://127.0.0.1:${port}`;
    const recovery = createLeaveRecovery();
    const factory = () => sharedLeaveCommands(signal=>createSessionApiClient(apiURL,()=> 'credential',signal),()=> 'stable-leave-operation', recovery);
    const commands = factory();
    const review=await commands.review('path');assert.equal(review.participant,true);
    assert.equal((await commands.submit(review,false)).kind,'failed');
    await assert.rejects(commands.submit(review,true));
    commands.dispose(); const resumed = factory(); const retained = await resumed.review('path');
    assert.equal(retained.retainActivity, false); await assert.rejects(resumed.submit(retained,true));
    wrongReceipt=true;assert.equal((await resumed.submit(retained,false)).kind,'failed');wrongReceipt=false;
    assert.equal((await resumed.submit(retained,false)).kind,'applied');assert.deepEqual(writes[0],writes[2]);
    assert.deepEqual(writes[0].body,{confirmed:true,retainActivity:false});
    assert.equal((await resumed.submit(retained,false)).kind,'superseded');
    const denied=sharedLeaveCommands(signal=>createSessionApiClient(apiURL,()=> 'credential',signal),()=> 'another-leave-operation');
    allowed=false;await assert.rejects(denied.review('path'));allowed=true;archived=true;await assert.rejects(denied.review('path'));denied.dispose();
    resumed.dispose();const count=writes.length;assert.equal((await resumed.submit(retained,false)).kind,'superseded');assert.equal(writes.length,count);
  }finally{server.close();server.closeAllConnections();}
});
