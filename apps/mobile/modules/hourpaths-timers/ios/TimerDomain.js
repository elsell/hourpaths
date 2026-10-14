"use strict";
var HourPathsTimerDomain = (() => {
  var __defProp = Object.defineProperty;
  var __getOwnPropDesc = Object.getOwnPropertyDescriptor;
  var __getOwnPropNames = Object.getOwnPropertyNames;
  var __hasOwnProp = Object.prototype.hasOwnProperty;
  var __export = (target, all) => {
    for (var name in all)
      __defProp(target, name, { get: all[name], enumerable: true });
  };
  var __copyProps = (to, from, except, desc) => {
    if (from && typeof from === "object" || typeof from === "function") {
      for (let key of __getOwnPropNames(from))
        if (!__hasOwnProp.call(to, key) && key !== except)
          __defProp(to, key, { get: () => from[key], enumerable: !(desc = __getOwnPropDesc(from, key)) || desc.enumerable });
    }
    return to;
  };
  var __toCommonJS = (mod) => __copyProps(__defProp({}, "__esModule", { value: true }), mod);

  // packages/client-core/src/native-timer-action.ts
  var native_timer_action_exports = {};
  __export(native_timer_action_exports, {
    applyNativeTimerAction: () => applyNativeTimerAction,
    refreshNativeTimerPresentation: () => refreshNativeTimerPresentation
  });

  // packages/client-core/src/tracking-timer-commands.ts
  function startTrackingTimer(state, pathId, id, operationId, startedAt) {
    const path = state.paths.find((candidate) => candidate.id === pathId);
    if (!path || state.unavailablePaths?.includes(pathId)) throw new Error("tracking_path_unavailable");
    if (state.timers.some((timer2) => timer2.pathId === pathId)) throw new Error("tracking_timer_already_running");
    const timer = { id, pathId, startedAt, timeZone: path.timeZone };
    state.timers.push(timer);
    state.operations.push({ ...timer, timerId: id, operationId, kind: "start" });
    return { ...timer };
  }
  function stopTrackingTimer(state, timerId, operationId, endedAt) {
    const timer = state.timers.find((candidate) => candidate.id === timerId);
    if (!timer) throw new Error("tracking_timer_unavailable");
    state.timers = state.timers.filter((candidate) => candidate.id !== timerId);
    if (Date.parse(endedAt) <= Date.parse(timer.startedAt)) {
      state.corrections.push({ timer, endedAt });
    } else {
      state.operations.push({ ...timer, timerId, operationId, kind: "stop", endedAt });
      if (Date.parse(endedAt) - Date.parse(timer.startedAt) < 1e3) state.notices.push({ id: operationId, reason: "subsecond", pathId: timer.pathId });
    }
  }

  // packages/client-core/src/native-timer-action.ts
  function applyNativeTimerAction(snapshot, action, now) {
    if (!action.owner || snapshot.owner !== action.owner) throw new Error("tracking_owner_mismatch");
    if (!Number.isSafeInteger(action.revision) || action.revision !== snapshot.revision) throw new Error("tracking_native_action_stale");
    if (!Number.isFinite(now) || !action.pathId || !action.timerId || !action.operationId || !["start", "stop"].includes(action.kind)) throw new Error("tracking_native_action_invalid");
    if (!snapshot.paths.some((path) => path.id === action.pathId) || snapshot.unavailablePaths?.includes(action.pathId)) {
      throw new Error("tracking_path_unavailable");
    }
    const state = JSON.parse(JSON.stringify(snapshot));
    const instant = new Date(now).toISOString();
    if (action.kind === "start") {
      if (state.operations.some((operation) => operation.operationId === action.operationId || operation.timerId === action.timerId)) {
        throw new Error("tracking_native_action_stale");
      }
      startTrackingTimer(state, action.pathId, action.timerId, action.operationId, instant);
    } else {
      if (!state.timers.some((timer) => timer.id === action.timerId && timer.pathId === action.pathId)) throw new Error("tracking_timer_unavailable");
      stopTrackingTimer(state, action.timerId, action.operationId, instant);
    }
    delete state.historyRefreshRange;
    state.revision++;
    return state;
  }
  function refreshNativeTimerPresentation(presentation, state, now, identifiers) {
    if (presentation.owner !== state.owner) throw new Error("tracking_owner_mismatch");
    const timers = presentation.timers.map((row, index) => {
      const active = state.timers.find((timer) => timer.pathId === row.pathId);
      const wasStopped = typeof row.stoppedAt === "number";
      const timerId = active?.id ?? identifiers[index * 2];
      const operationId = identifiers[index * 2 + 1];
      if (!timerId || !operationId) throw new Error("tracking_native_action_invalid");
      const previousGoal = row.goal;
      const gap = active && wasStopped ? Date.parse(active.startedAt) - Number(row.stoppedAt) : 0;
      const resumedGoal = active && wasStopped && previousGoal?.lower !== void 0 && previousGoal.upper !== void 0 && now < previousGoal.upper && gap >= 0 ? {
        ...previousGoal,
        lower: previousGoal.lower + gap,
        progressUpper: previousGoal.progressUpper === void 0 ? void 0 : previousGoal.progressUpper + gap
      } : void 0;
      const target = JSON.stringify({ lease: presentation.accountLease, action: {
        owner: state.owner,
        revision: state.revision,
        pathId: row.pathId,
        timerId,
        operationId,
        kind: active ? "stop" : "start"
      } });
      return {
        ...row,
        id: timerId,
        ...active ? {
          startedAt: Date.parse(active.startedAt),
          stoppedAt: void 0,
          ...wasStopped ? { goal: resumedGoal } : {}
        } : { stoppedAt: row.stoppedAt ?? now },
        action: { target, label: active ? presentation.stopLabel : presentation.startLabel }
      };
    });
    const titles = presentation.titles;
    return { ...presentation, timers, title: titles?.[state.timers.length] ?? presentation.title };
  }
  return __toCommonJS(native_timer_action_exports);
})();
