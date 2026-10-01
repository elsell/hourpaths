import type { Audience, AudiencePreference, CommandResult, Eligibility, Preset } from '../domain/nudge';
export interface NudgeCommands {
  send(pathId: string, recipientId: string, preset: Preset, signal?: AbortSignal): Promise<CommandResult<void>>;
  save(preference: AudiencePreference, audience: Audience, signal?: AbortSignal): Promise<CommandResult<AudiencePreference>>;
  dispose(): void;
}
export interface NudgesRepository {
  eligibility(pathId: string, recipientId: string, signal?: AbortSignal): Promise<Eligibility>;
  audience(pathId: string, signal?: AbortSignal): Promise<AudiencePreference>;
  commands(key: () => string): NudgeCommands;
}
