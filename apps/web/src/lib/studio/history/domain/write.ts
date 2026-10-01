export interface ActivityInput { readonly localDate: string; readonly localTime: string; readonly seconds: number; readonly note: string }
export interface ActivityWrite { readonly pathId: string; readonly activityId: string | null; readonly operationId: string; readonly input: ActivityInput }
export interface ActivityFormDefaults { readonly pathName: string; readonly currentInstant: number; readonly timeZone: string; readonly canTrack: boolean }
