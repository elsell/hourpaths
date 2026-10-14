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

  // packages/client-core/src/offline-recorded-activity.ts
  function recordedActivityChanges(state) {
    const entries = /* @__PURE__ */ new Map();
    for (const operation of state.activityOperations ?? []) {
      const id = activityIdentity(state, operation.activity.id);
      const prior = entries.get(id);
      entries.set(id, { previous: prior ? prior.previous : operation.previous, activity: { ...operation.activity, id } });
    }
    return [...entries.values()];
  }
  function recordedActivityDelta(state, pathId, period, includeAcknowledged = true) {
    const duration = (entry) => entry ? Math.max(0, Math.floor((Math.min(Date.parse(entry.endedAt), period?.endsAt ?? Infinity) - Math.max(Date.parse(entry.startedAt), period?.startsAt ?? -Infinity)) / 1e3)) : 0;
    return [...includeAcknowledged ? state.activityDeltas ?? [] : [], ...recordedActivityChanges(state)].filter((value) => (value.activity?.pathId ?? value.previous?.pathId) === pathId).reduce((total, value) => total + duration(value.activity) - duration(value.previous), 0);
  }
  function activityIdentity(state, id) {
    return state.activityAliases?.[id] ?? id;
  }

  // packages/client-core/src/calendar-time.ts
  var formatters = /* @__PURE__ */ new Map();
  var localDatePattern = /^(\d{4})-(\d{2})-(\d{2})$/;
  var localTimePattern = /^(\d{2}):(\d{2}):(\d{2})$/;
  function localDateTimeValue(value) {
    const date = localDatePattern.exec(value.localDate);
    const time = localTimePattern.exec(value.localTime);
    if (!date || !time) return void 0;
    const year = Number(date[1]);
    const month = Number(date[2]);
    const day = Number(date[3]);
    const hour = Number(time[1]);
    const minute = Number(time[2]);
    const second = Number(time[3]);
    const nominal = /* @__PURE__ */ new Date(0);
    nominal.setUTCFullYear(year, month - 1, day);
    nominal.setUTCHours(hour, minute, second, 0);
    if (nominal.getUTCFullYear() !== year || nominal.getUTCMonth() !== month - 1 || nominal.getUTCDate() !== day || nominal.getUTCHours() !== hour || nominal.getUTCMinutes() !== minute || nominal.getUTCSeconds() !== second) return void 0;
    return nominal.getTime();
  }
  function participantLocalDateTime(value, timeZone) {
    if (!Number.isFinite(value) || !timeZone || timeZone === "Local" || timeZone.trim() !== timeZone) return void 0;
    try {
      let formatter = formatters.get(timeZone);
      if (!formatter) {
        formatter = new Intl.DateTimeFormat("en-CA", {
          timeZone,
          calendar: "iso8601",
          numberingSystem: "latn",
          year: "numeric",
          month: "2-digit",
          day: "2-digit",
          hour: "2-digit",
          minute: "2-digit",
          second: "2-digit",
          hourCycle: "h23"
        });
        if (formatters.size >= 32) formatters.delete(formatters.keys().next().value);
        formatters.set(timeZone, formatter);
      }
      const parts = formatter.formatToParts(new Date(value));
      const part = (type) => parts.find((candidate) => candidate.type === type)?.value;
      const local = {
        localDate: `${part("year")}-${part("month")}-${part("day")}`,
        localTime: `${part("hour")}:${part("minute")}:${part("second")}`
      };
      return localDateTimeValue(local) === void 0 ? void 0 : local;
    } catch {
      return void 0;
    }
  }
  function participantInstantValue(value, timeZone) {
    const wallValue = localDateTimeValue(value);
    if (wallValue === void 0) return void 0;
    const offsets = /* @__PURE__ */ new Set();
    for (let hour = -72; hour <= 72; hour += 1) {
      const sample = wallValue + hour * 36e5;
      const localSample = participantLocalDateTime(sample, timeZone);
      const localSampleValue = localSample && localDateTimeValue(localSample);
      if (localSampleValue !== void 0) offsets.add(localSampleValue - sample);
    }
    const exact = [];
    const forward = [];
    for (const offset of offsets) {
      const instant = wallValue - offset;
      const candidate = participantLocalDateTime(instant, timeZone);
      const candidateWall = candidate && localDateTimeValue(candidate);
      if (candidateWall === wallValue) exact.push(instant);
      else if (candidateWall !== void 0 && candidateWall > wallValue) forward.push({ instant, wall: candidateWall });
    }
    if (exact.length > 0) return Math.min(...exact);
    forward.sort((left, right) => left.wall - wallValue - (right.wall - wallValue) || left.instant - right.instant);
    return forward[0]?.instant;
  }

  // packages/client-core/src/calendar-goal.ts
  function wall(year, month, day, hour = 0, minute = 0) {
    const value = /* @__PURE__ */ new Date(0);
    value.setUTCFullYear(year, month, day);
    value.setUTCHours(hour, minute, 0, 0);
    return value;
  }
  function days(year, month) {
    return wall(year, month + 1, 0).getUTCDate();
  }
  function currentGoalPeriod(goal, zone, now) {
    const invalid = () => new Error("tracking_goal_invalid");
    const a = { minute: goal.alignment.minute ?? 0, hour: goal.alignment.hour ?? 0, isoWeekday: goal.alignment.isoWeekday ?? 0, month: goal.alignment.month ?? 0, day: goal.alignment.day ?? 0 };
    const range = (n, min, max) => Number.isInteger(n) && n >= min && n <= max;
    const valid = Object.values(a).every(Number.isInteger) && Number.isSafeInteger(goal.targetSeconds) && goal.targetSeconds > 0 && (goal.recurrence === "hourly" ? range(a.minute, 0, 59) && !a.hour && !a.isoWeekday && !a.month && !a.day : goal.recurrence === "daily" ? range(a.hour, 0, 23) && !a.minute && !a.isoWeekday && !a.month && !a.day : goal.recurrence === "weekly" ? range(a.isoWeekday, 1, 7) && !a.minute && !a.hour && !a.month && !a.day : goal.recurrence === "monthly" ? range(a.day, 1, 31) && !a.minute && !a.hour && !a.isoWeekday && !a.month : goal.recurrence === "yearly" ? range(a.month, 1, 12) && range(a.day, 1, days(2e3, a.month - 1)) && !a.minute && !a.hour && !a.isoWeekday : false);
    if (!valid) throw invalid();
    const local = participantLocalDateTime(now, zone);
    if (!local) throw invalid();
    const nominal = localDateTimeValue(local);
    if (nominal === void 0) throw invalid();
    const date = new Date(nominal), year = date.getUTCFullYear(), month = date.getUTCMonth(), day = date.getUTCDate();
    const instants = /* @__PURE__ */ new Set();
    for (let offset = -3; offset <= 3; offset++) {
      let boundary;
      switch (goal.recurrence) {
        case "hourly":
          boundary = wall(year, month, day, date.getUTCHours() + offset, a.minute);
          break;
        case "daily":
          boundary = wall(year, month, day + offset, a.hour);
          break;
        case "weekly":
          boundary = wall(year, month, day + a.isoWeekday - (date.getUTCDay() || 7) + offset * 7);
          break;
        case "monthly": {
          const shifted = wall(year, month + offset, 1);
          boundary = wall(shifted.getUTCFullYear(), shifted.getUTCMonth(), Math.min(a.day, days(shifted.getUTCFullYear(), shifted.getUTCMonth())));
          break;
        }
        case "yearly":
          boundary = wall(year + offset, a.month - 1, Math.min(a.day, days(year + offset, a.month - 1)));
          break;
      }
      const iso = boundary.toISOString();
      const instant = participantInstantValue({ localDate: iso.slice(0, 10), localTime: iso.slice(11, 19) }, zone);
      if (instant === void 0) throw invalid();
      instants.add(instant);
    }
    const ordered = [...instants].sort((a2, b) => a2 - b);
    const next = ordered.findIndex((instant) => instant > now);
    if (next <= 0) throw invalid();
    return { startsAt: ordered[next - 1], endsAt: ordered[next], targetSeconds: goal.targetSeconds };
  }

  // packages/client-core/src/offline-tracking.ts
  function projectTrackingState(state, pathId, now, periodFor = (path) => path.goal ? currentGoalPeriod(path.goal, path.timeZone, now) : null) {
    if (state.unavailablePaths?.includes(pathId)) return null;
    const summary = Object.hasOwn(state.summaries, pathId) ? state.summaries[pathId] : void 0;
    if (!summary) return null;
    const pending = state.operations.filter((value) => value.pathId === pathId);
    const intervals = [
      ...state.unreflected.filter((value) => value.pathId === pathId),
      ...pending.filter((value) => value.kind !== "start" && value.endedAt).map((value) => ({ startedAt: value.correctedStartedAt ?? value.startedAt, endedAt: value.endedAt }))
    ];
    const seconds = (start, end) => Math.max(0, Math.floor((end - start) / 1e3));
    const total = intervals.reduce((sum, value) => sum + seconds(Date.parse(value.startedAt), Date.parse(value.endedAt)), 0);
    const path = state.paths.find((value) => value.id === pathId);
    let period = summary.period;
    let periodIntervals = intervals;
    let includeAcknowledgedEdits = true;
    if (path && path.goal !== void 0) {
      const bounds = periodFor(path);
      if (!bounds) period = null;
      else if (summary.period?.startsAt === bounds.startsAt && summary.period.endsAt === bounds.endsAt) {
        period = { ...bounds, savedSeconds: summary.period.savedSeconds };
      } else {
        const recorded = new Map([...state.unreflected, ...state.history].filter((value) => value.pathId === pathId).map((value) => [value.id, value]));
        periodIntervals = [...recorded.values(), ...pending.filter((value) => value.kind !== "start" && value.endedAt).map((value) => ({ startedAt: value.correctedStartedAt ?? value.startedAt, endedAt: value.endedAt }))];
        period = { ...bounds, savedSeconds: 0 };
        includeAcknowledgedEdits = false;
      }
    }
    const periodDelta = period ? periodIntervals.reduce((sum, value) => sum + seconds(Math.max(period.startsAt, Date.parse(value.startedAt)), Math.min(period.endsAt, Date.parse(value.endedAt))), 0) : 0;
    const timer = state.timers.find((value) => value.pathId === pathId);
    return {
      savedTotalSeconds: summary.savedTotalSeconds + total + recordedActivityDelta(state, pathId),
      period: period ? { ...period, savedSeconds: period.savedSeconds + periodDelta + recordedActivityDelta(state, pathId, period, includeAcknowledgedEdits) } : null,
      activeSession: timer ? { id: timer.id, startedAt: Date.parse(timer.startedAt), originalStartedAt: timer.startedAt, timeZone: timer.timeZone } : null,
      pending: (state.activityOperations ?? []).some((operation) => operation.activity.pathId === pathId) || pending.some((operation) => !state.corrections.some((value) => value.timer.id === operation.timerId))
    };
  }

  // packages/client-core/src/native-goal-period.ts
  var maximumProjectionWindow = 8 * 60 * 60 * 1e3;
  function nativeGoalPeriodSchedule(state, owner, pathId, now, expiresAt) {
    if (!owner || state.owner !== owner) throw new Error("tracking_owner_mismatch");
    if (!Number.isFinite(now) || !Number.isFinite(expiresAt)) throw new Error("tracking_native_clock_invalid");
    const end = Math.min(expiresAt, now + maximumProjectionWindow);
    const periods = [];
    let cursor = now;
    while (cursor < end) {
      const period = projectTrackingState(state, pathId, cursor)?.period;
      if (!period || period.startsAt > cursor || period.endsAt <= cursor) break;
      if (![period.startsAt, period.endsAt, period.savedSeconds, period.targetSeconds].every(Number.isFinite) || period.savedSeconds < 0 || period.targetSeconds <= 0) throw new Error("tracking_goal_invalid");
      if (periods.length >= 32) throw new Error("tracking_native_period_limit");
      periods.push({ ...period });
      cursor = period.endsAt;
    }
    return periods;
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
        ...typeof presentation.activityExpiresAt === "number" ? {
          goalPeriods: nativeGoalPeriodSchedule(state, presentation.owner, row.pathId, now, presentation.activityExpiresAt)
        } : {},
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
