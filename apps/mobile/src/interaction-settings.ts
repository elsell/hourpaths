export type InteractionSettings = Readonly<{
  commentsEnabled: boolean;
  reactionsEnabled: boolean;
}>;

export type InteractionSetting = keyof InteractionSettings;

export type FrozenInteractionSettingsIntent = Readonly<{
  idempotencyKey: string;
  settings: InteractionSettings;
}>;

function sameSettings(left: InteractionSettings, right: InteractionSettings) {
  return left.commentsEnabled === right.commentsEnabled &&
    left.reactionsEnabled === right.reactionsEnabled;
}

export function interactionSettingsWithChange(
  current: InteractionSettings,
  setting: InteractionSetting,
  enabled: boolean,
): InteractionSettings {
  return { ...current, [setting]: enabled };
}

export function interactionSettingsFromAPI(value: unknown): InteractionSettings {
  if (!value || typeof value !== 'object') throw new Error('invalid_interaction_settings');
  const candidate = value as Record<string, unknown>;
  if (
    Object.keys(candidate).sort().join(',') !== 'commentsEnabled,reactionsEnabled' ||
    typeof candidate.commentsEnabled !== 'boolean' ||
    typeof candidate.reactionsEnabled !== 'boolean'
  ) {
    throw new Error('invalid_interaction_settings');
  }
  return {
    commentsEnabled: candidate.commentsEnabled,
    reactionsEnabled: candidate.reactionsEnabled,
  };
}

export function createInteractionSettingsIntentCoordinator(createKey: () => string) {
  let frozen: FrozenInteractionSettingsIntent | undefined;
  return {
    complete(intent: FrozenInteractionSettingsIntent) {
      if (frozen === intent) frozen = undefined;
    },
    freeze(settings: InteractionSettings): FrozenInteractionSettingsIntent {
      if (frozen && sameSettings(frozen.settings, settings)) return frozen;
      frozen = { idempotencyKey: createKey(), settings: { ...settings } };
      return frozen;
    },
    invalidate() {
      frozen = undefined;
    },
  };
}
