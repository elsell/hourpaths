export type ManualActivityPresentationOwner = 'path-details' | 'activity-details';

export function ownsManualActivityPresentation(
  owner: ManualActivityPresentationOwner | null,
  route: ManualActivityPresentationOwner,
) {
  return owner === route;
}

export function dismissManualActivityPresentation(
  owner: ManualActivityPresentationOwner | null,
  route: ManualActivityPresentationOwner,
): ManualActivityPresentationOwner | null {
  return owner === route ? null : owner;
}
