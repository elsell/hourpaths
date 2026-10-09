import { AccountExportFailure, type AccountExportSink } from '@hourpaths/client-core';

/** User-initiated, local-only JSON download. No API or arbitrary destination. */
export const browserAccountExportJSON: AccountExportSink = {
  async save(document, current) {
    if (!current()) throw new AccountExportFailure('account_changed');
    const filename = 'hourpaths-export-' + new Date(document.collectionCompletedAt).toISOString().replace(/[^0-9]/g, '') + '.json';
    const blob = new Blob([JSON.stringify(document, null, 2) + '\n'], { type: 'application/json;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    try {
      if (!current()) throw new AccountExportFailure('account_changed');
      const link = window.document.createElement('a');
      link.href = url; link.download = filename;
      window.document.body.appendChild(link);
      try { link.click(); } finally { link.remove(); }
    } finally {
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    }
    return 'saved';
  },
};
