export type DeviceDefaultsSource = {
  timeZone(): string;
  locale(): string;
  firstDay(locale: string): number | undefined;
};

export const browserDeviceDefaultsSource: DeviceDefaultsSource = {
  timeZone: () => Intl.DateTimeFormat().resolvedOptions().timeZone,
  locale: () => {
    const candidates = [
      ...(typeof navigator === 'undefined' ? [] : navigator.languages),
      Intl.DateTimeFormat().resolvedOptions().locale,
    ];
    for (const candidate of candidates) {
      try {
        if (candidate && new Intl.Locale(candidate).region) return candidate;
      } catch { /* Continue to the resolved regional locale. */ }
    }
    return '';
  },
  firstDay: (locale) => {
    const localeWithWeekInfo = new Intl.Locale(locale) as Intl.Locale & {
      getWeekInfo?: () => { firstDay: number };
      weekInfo?: { firstDay: number };
    };
    return localeWithWeekInfo.getWeekInfo?.().firstDay ?? localeWithWeekInfo.weekInfo?.firstDay;
  },
};
