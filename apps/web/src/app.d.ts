import type { SupportedLocale } from '@hourpaths/i18n';

declare global {
  namespace App {
    interface Locals {
      locale: SupportedLocale;
    }
  }
}

export {};
