import { createSessionApiClient } from '@hourpaths/api-client';
import { createNudgeSendOperationOwner, createNudgeAudienceOperationOwner, nudgeEligibilityFromAPI, nudgeAudiencePreferenceFromAPI, reviewNudgeSend, reviewNudgeAudienceChange } from '@hourpaths/client-core';
import { NudgeFailure } from '../domain/nudge';
import type { NudgesRepository } from '../ports/nudges-repository';
function required<T>(result: { data?: { data: T }; response: Response }): T {
  if (!result.response.ok || !result.data) throw new NudgeFailure(result.response.status === 409 ? 'conflict' : 'unavailable');
  return result.data.data;
}
export function apiNudgesRepository(baseURL: string, token: () => string | null, rejected: (token: string | null) => void): NudgesRepository {
  const client = (signal?: AbortSignal) => createSessionApiClient(baseURL, token, signal, rejected);
  return {
    async eligibility(pathId, recipientId, signal) {
      const value = nudgeEligibilityFromAPI(required(await client(signal).getPathMemberNudgeEligibility(pathId, recipientId)));
      if (value.pathId !== pathId || value.recipientUserId !== recipientId) throw new NudgeFailure();
      return value;
    },
    async audience(pathId, signal) {
      const value = nudgeAudiencePreferenceFromAPI(required(await client(signal).getPathNudgePreference(pathId)));
      if (value.pathId !== pathId) throw new NudgeFailure();
      return value;
    },
    commands(key) {
      const sends = createNudgeSendOperationOwner(key), saves = createNudgeAudienceOperationOwner(key);
      let disposed = false;
      return {
        dispose() { disposed = true; sends.cancel(); saves.cancel(); },
        async send(pathId, recipientId, preset, signal) {
          if (disposed) return { kind: 'superseded' };
          const result = await sends.submit(reviewNudgeSend(pathId, recipientId, preset), async (path, recipient, body, id) => required(await client(signal).sendPathMemberNudge(path, recipient, body, id)));
          if (result.kind === 'applied') return { kind: 'applied', value: undefined };
          if (result.kind === 'failed') return { kind: 'failed', conflict: result.cause instanceof NudgeFailure && result.cause.kind === 'conflict' };
          return result;
        },
        async save(preference, audience, signal) {
          if (disposed) return { kind: 'superseded' };
          const review = reviewNudgeAudienceChange(preference, audience);
          if (!review.changed) return { kind: 'applied', value: preference };
          const result = await saves.submit(preference.pathId, review, async (path, body, id) => required(await client(signal).updatePathNudgePreference(path, body, id)));
          if (result.kind === 'applied') return { kind: 'applied', value: result.preference };
          if (result.kind === 'failed') return { kind: 'failed', conflict: result.cause instanceof NudgeFailure && result.cause.kind === 'conflict' };
          return result;
        },
      };
    },
  };
}
