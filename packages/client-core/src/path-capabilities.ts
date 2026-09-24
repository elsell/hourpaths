export type PathCapabilities = Readonly<{
  trackTime: boolean;
  renamePath: boolean;
  inviteMembers: boolean;
  manageMembers?: boolean;
  leavePath?: boolean;
  manageGoals: boolean;
  manageLifecycle: boolean;
  manageVisibility?: boolean;
  transferOwnership: boolean;
}>;

export type CapabilityPath = Readonly<{
  id: string;
  archivedAt?: string | null;
  capabilities: PathCapabilities;
}>;

const noCapabilities: PathCapabilities = Object.freeze({
  trackTime: false,
  renamePath: false,
  inviteMembers: false,
  manageMembers: false,
  manageGoals: false,
  manageLifecycle: false,
  manageVisibility: false,
  transferOwnership: false,
});

export function effectivePathCapabilities(path: CapabilityPath): PathCapabilities {
  const archived = path.archivedAt !== undefined && path.archivedAt !== null;
  if (!archived) return { ...path.capabilities };
  return {
    ...noCapabilities,
    ...(path.capabilities.leavePath === true ? { leavePath: true } : {}),
    manageLifecycle: path.capabilities.manageLifecycle,
  };
}

export function pathsRequiringTimerRestore<P extends CapabilityPath>(paths: readonly P[]): P[] {
  return paths.filter((path) => effectivePathCapabilities(path).trackTime);
}
