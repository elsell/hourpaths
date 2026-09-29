import assert from 'node:assert/strict';
import test from 'node:test';
import { createServer, type ServerResponse } from 'node:http';
import { SessionController } from './studio/session/application/session-controller';
import type { Session } from './studio/session/domain/session';
import { apiPathRepository } from './studio/paths/adapters/api-path-repository';
import { apiSocialRepository } from './studio/social/adapters/api-social-repository';
import { apiPreferencesRepository } from './studio/preferences/adapters/api-preferences-repository';
import { apiStatisticsRepository } from './studio/analytics/adapters/api-statistics-repository';
import { apiHistorySource } from './studio/history/adapters/api-history-source';

test('every Studio transport preserves a rotated session after a late 401 and rejects a current credential', async context => {
  let receive!: (response: ServerResponse) => void;
  let authorization: string | undefined;
  const server = createServer((request, response) => { authorization = request.headers.authorization; receive(response); });
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
  context.after(() => { server.closeAllConnections(); server.close(); });
  const address = server.address(); assert.ok(address && typeof address !== 'string');
  const url = `http://127.0.0.1:${address.port}`;
  for (const surface of ['paths', 'social', 'preferences', 'statistics', 'history'] as const) {
    await context.test(surface, async () => {
      let stored: Session | null = { token: 'original', expiresAt: 1_000_000, destination: 'home' }, lost = 0;
      const controller = new SessionController(stored, { read: () => stored, write: value => { stored = value; }, clear: () => { stored = null; } }, {
        refresh: async value => ({ ...value, token: 'replacement' }), revoke: async () => undefined,
      }, () => 0, () => { lost++; });
      const token = () => controller.token(), rejected = (credential: string | null) => controller.reject(credential);
      const paths = apiPathRepository(url, token, rejected);
      const request = () => surface === 'paths' ? paths.list(false)
        : surface === 'social' ? apiSocialRepository(url, token, rejected).viewer()
        : surface === 'preferences' ? apiPreferencesRepository(url, token, rejected).identity()
        : surface === 'statistics' ? apiStatisticsRepository(url, token, rejected).load({ range: 'week', pathIds: [] })
        : apiHistorySource(url, token, paths, rejected).initial();
      const respond = async (status: number, rotate = false) => {
        const arrived = new Promise<ServerResponse>(resolve => { receive = resolve; });
        const pending = assert.rejects(request());
        const response = await arrived;
        assert.equal(authorization, `Bearer ${rotate ? 'original' : 'replacement'}`);
        if (rotate) await controller.refresh();
        response.writeHead(status, { 'Content-Type': 'application/json' }); response.end('{}');
        await pending;
      };
      await respond(401, true);
      assert.equal(controller.token(), 'replacement'); assert.equal(lost, 0);
      await respond(503); await respond(403);
      assert.equal(controller.token(), 'replacement'); assert.equal(lost, 0);
      await respond(401);
      assert.equal(stored, null); assert.equal(lost, 1);
    });
  }
});
