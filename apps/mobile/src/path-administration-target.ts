export type PathAdministrationCredential = Readonly<{ token: string }>;

export type PathAdministrationTarget<Credential extends PathAdministrationCredential> = {
  ownerID: string;
  pathID: string;
  session: Credential;
  sessionTokens: string[];
};

export function createPathAdministrationTarget<Credential extends PathAdministrationCredential>(
  ownerID: string,
  pathID: string,
  session: Credential,
): PathAdministrationTarget<Credential> {
  return { ownerID, pathID, session, sessionTokens: [session.token] };
}

export function rotatePathAdministrationTarget<Credential extends PathAdministrationCredential>(
  target: PathAdministrationTarget<Credential>,
  ownerID: string,
  session: Credential,
): boolean {
  if (target.ownerID !== ownerID) return false;
  if (!target.sessionTokens.includes(session.token)) target.sessionTokens.push(session.token);
  return true;
}

export function ownsPathAdministrationTarget<Credential extends PathAdministrationCredential>(
  target: PathAdministrationTarget<Credential> | null,
  ownerID: string,
  pathID: string,
  sessionToken: string,
): target is PathAdministrationTarget<Credential> {
  return target?.ownerID === ownerID && target.pathID === pathID && target.sessionTokens.includes(sessionToken);
}

export function currentPathAdministrationCredential<Credential extends PathAdministrationCredential>(
  target: PathAdministrationTarget<Credential> | null,
  ownerID: string | null,
  pathID: string,
  current: Credential | null,
): Credential | null {
  return current && ownerID && ownsPathAdministrationTarget(target, ownerID, pathID, current.token)
    ? current
    : null;
}
