type NotificationSessionCredential = Readonly<{ token: string }>;

export type NotificationSessionTarget<Credential extends NotificationSessionCredential> = Readonly<{
  intentKey: string;
  ownerID: string;
  requestSession: Credential;
  sessionTokens: readonly string[];
}>;

export function createNotificationSessionTarget<Credential extends NotificationSessionCredential>(
  ownerID: string,
  requestSession: Credential,
  intentKey: string,
): NotificationSessionTarget<Credential> {
  return { intentKey, ownerID, requestSession, sessionTokens: [requestSession.token] };
}

export function rotateNotificationSessionTarget<Credential extends NotificationSessionCredential>(
  target: NotificationSessionTarget<Credential>,
  ownerID: string,
  session: Credential,
): NotificationSessionTarget<Credential> {
  if (target.ownerID !== ownerID || target.sessionTokens.includes(session.token)) return target;
  return { ...target, sessionTokens: [...target.sessionTokens, session.token] };
}

export function ownsNotificationSessionTarget<Credential extends NotificationSessionCredential>(
  target: NotificationSessionTarget<Credential> | null,
  ownerID: string | null | undefined,
  sessionToken: string | null | undefined,
  intentKey: string,
): target is NotificationSessionTarget<Credential> {
  return target !== null && target.ownerID === ownerID && target.intentKey === intentKey &&
    target.sessionTokens.includes(sessionToken ?? '');
}

export function currentNotificationSessionCredential<Credential extends NotificationSessionCredential>(
  target: NotificationSessionTarget<Credential> | null,
  ownerID: string | null | undefined,
  intentKey: string,
  current: Credential | null,
): Credential | null {
  return current && ownsNotificationSessionTarget(target, ownerID, current.token, intentKey) ? current : null;
}
