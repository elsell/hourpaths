import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { once } from 'node:events';
import { apiSharingRepository } from './studio/sharing/adapters/api-sharing-repository';
test('member review binds current identity and role; destructive retries preserve the operation and reject mismatched receipts', async () => {
  const calls: { key: string; body: unknown }[] = [];
  let wrongIdentity = false, wrongReceipt = false, allowed = true;
  const member = { userId: 'person', username: 'person', displayName: 'person', role: 'participant', sessionCount: 3, totalTrackedSeconds: 900, runningTimer: true, canRemove: true, canChangeRole: true, canGrantAdministrator: true };
  const server = createServer(async (request, response) => {
    response.setHeader('Content-Type', 'application/json'); let data: unknown = member;
    if (request.method === 'PATCH') {
      let body = ''; for await (const part of request) body += part;
      calls.push({ key: request.headers['idempotency-key'] as string, body: JSON.parse(body) });
      response.statusCode = calls.length === 1 ? 503 : 200;
      data = { pathId: 'path', userId: wrongReceipt ? 'other' : 'person', role: 'supporter', activityDeleted: true };
    } else if (request.url?.includes('removal-review')) data = { ...member, userId: wrongIdentity ? 'other' : 'person' };
    else data = [{ ...member, canRemove: allowed, canChangeRole: allowed }];
    response.end(JSON.stringify({ data, meta: { nextCursor: '' } }));
  }); server.listen(0, '127.0.0.1'); await once(server, 'listening');
  try {
    const repo = apiSharingRepository(`http://127.0.0.1:${(server.address() as { port: number }).port}`, () => 'credential', () => {});
    const commands = repo.memberCommands(() => 'stable-role-change-key');
    const review = await commands.review('path', 'person', 'supporter');
    assert.equal(review.member.runningTimer, true);
    assert.equal((await commands.submit(review)).kind, 'failed');
    assert.equal((await commands.submit(review)).kind, 'applied');
    assert.deepEqual(calls[0], calls[1]);
    assert.deepEqual(calls[0]?.body, { confirmed: true, expectedRole: 'participant', role: 'supporter' });
    wrongReceipt = true;
    const changedReview = await commands.review('path', 'person', 'supporter');
    assert.equal((await commands.submit(changedReview)).kind, 'failed');
    allowed = false; await assert.rejects(commands.review('path', 'person', 'remove')); allowed = true;
    wrongIdentity = true;
    await assert.rejects(commands.review('path', 'person', 'remove'));
    commands.dispose();
    await assert.rejects(commands.review('path', 'person', 'remove'));
  } finally { server.close(); server.closeAllConnections(); }
});
test('member list preserves Path progress and viewer-owned block state while rejecting invalid progress', async () => {
  let progress = { accumulatedSeconds: 120, targetSeconds: 300 };
  const server = createServer((_request,response) => {
    response.setHeader('Content-Type','application/json');
    response.end(JSON.stringify({ data:[{userId:'person',username:'person',displayName:'person',role:'participant',sessionCount:2,totalTrackedSeconds:600,blockedByViewer:true,intervalProgress:progress,overallProgress:{accumulatedSeconds:600,targetSeconds:900}}],meta:{nextCursor:''}}));
  });server.listen(0,'127.0.0.1');await once(server,'listening');
  try {
    const repo=apiSharingRepository(`http://127.0.0.1:${(server.address() as {port:number}).port}`,()=> 'credential',()=>{});
    const member=(await repo.members('path','')).items[0];
    assert.equal(member.blockedByViewer,true);assert.deepEqual(member.intervalProgress,progress);assert.deepEqual(member.overallProgress,{accumulatedSeconds:600,targetSeconds:900});
    progress={accumulatedSeconds:-1,targetSeconds:300};await assert.rejects(repo.members('path',''));
  }finally{server.close();server.closeAllConnections();}
});
