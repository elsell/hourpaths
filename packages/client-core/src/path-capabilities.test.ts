import assert from 'node:assert/strict';
import test from 'node:test';
import {
  effectivePathCapabilities,
  pathsRequiringTimerRestore,
  type CapabilityPath,
  type PathCapabilities,
} from './index.js';

const all: PathCapabilities = {
  trackTime: true,
  renamePath: true,
  inviteMembers: true,
  manageMembers: true,
  manageGoals: true,
  manageLifecycle: true,
  manageVisibility: true,
  transferOwnership: true,
};
const none: PathCapabilities = {
  trackTime: false,
  renamePath: false,
  inviteMembers: false,
  manageMembers: false,
  manageGoals: false,
  manageLifecycle: false,
  manageVisibility: false,
  transferOwnership: false,
};

test('effective Path capabilities follow only the authoritative projection', () => {
  const supporter: CapabilityPath = { id: 'supporter', capabilities: none };
  const participant: CapabilityPath = {
    id: 'participant',
    capabilities: { ...none, trackTime: true },
  };
  const administrator: CapabilityPath = {
    id: 'administrator',
    capabilities: { ...none, trackTime: true, renamePath: true, inviteMembers: true, manageGoals: true, manageMembers: true },
  };

  assert.deepEqual(effectivePathCapabilities(supporter), none);
  assert.deepEqual(effectivePathCapabilities(participant), { ...none, trackTime: true });
  assert.deepEqual(effectivePathCapabilities(administrator), {
    ...none,
    trackTime: true,
    renamePath: true,
    inviteMembers: true,
    manageMembers: true,
    manageGoals: true,
  });
});

test('archived Paths are read-only except for projected lifecycle management', () => {
  const archived: CapabilityPath = {
    id: 'archived',
    archivedAt: '2026-07-23T20:00:00Z',
    capabilities: all,
  };
  assert.deepEqual(effectivePathCapabilities(archived), {
    ...none,
    manageLifecycle: true,
  });
});

test('visibility management follows the active creator projection only', () => {
  const creator: CapabilityPath = { id: 'creator', capabilities: all };
  const administrator: CapabilityPath = { id: 'administrator', capabilities: none };
  assert.equal(effectivePathCapabilities(creator).manageVisibility, true);
  assert.equal(effectivePathCapabilities(administrator).manageVisibility, false);
});

test('timer restoration includes only active Paths explicitly allowed to track', () => {
  const paths: CapabilityPath[] = [
    { id: 'supporter', capabilities: none },
    { id: 'participant', capabilities: { ...none, trackTime: true } },
    { id: 'archived', archivedAt: '2026-07-23T20:00:00Z', capabilities: all },
  ];
  assert.deepEqual(pathsRequiringTimerRestore(paths), [paths[1]]);
});
