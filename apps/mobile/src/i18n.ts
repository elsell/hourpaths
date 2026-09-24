import { createTranslator } from '@hourpaths/i18n';

export type DeviceLocaleReader = () => readonly { languageTag: string }[];

export function createDeviceTranslator(readLocales: DeviceLocaleReader) {
  return createTranslator(readLocales().map(({ languageTag }) => languageTag));
}
