import { StyleSheet } from 'react-native';
import { mobileTheme } from './tokens';

export const mobileShellStyles = StyleSheet.create({
  sheetBody: {
    flex: 1,
  },
  emptyState: {
    gap: mobileTheme.spacing.sm,
    paddingVertical: mobileTheme.spacing.lg,
  },
  screen: {
    backgroundColor: mobileTheme.colors.background,
    flex: 1,
    paddingHorizontal: mobileTheme.spacing.md,
    paddingTop: mobileTheme.spacing.sm,
  },
  sheet: {
    backgroundColor: mobileTheme.colors.background,
    flex: 1,
  },
  sheetCompactContent: {
    gap: mobileTheme.spacing.lg,
    padding: mobileTheme.spacing.md,
    paddingBottom: mobileTheme.spacing.xxl,
  },
  sheetContent: {
    gap: mobileTheme.spacing.md,
    padding: mobileTheme.spacing.lg,
    paddingBottom: mobileTheme.spacing.xxl,
  },
  sheetHeader: {
    alignItems: 'center',
    borderBottomColor: mobileTheme.colors.surfaceRaised,
    borderBottomWidth: StyleSheet.hairlineWidth,
    flexDirection: 'row',
    minHeight: 52,
    paddingHorizontal: mobileTheme.spacing.xs,
  },
  sheetHeaderAction: {
    alignItems: 'flex-start',
    flexShrink: 1,
    maxWidth: '45%',
    minWidth: 72,
  },
  sheetHeaderStacked: {
    alignItems: 'stretch',
    flexDirection: 'column',
    gap: mobileTheme.spacing.xxs,
    paddingTop: mobileTheme.spacing.sm,
  },
  sheetHeaderStackedActions: {
    alignItems: 'center',
    flexDirection: 'row',
    justifyContent: 'space-between',
    width: '100%',
  },
  sheetHeaderTrailing: {
    alignItems: 'flex-end',
  },
  sheetTitle: {
    color: mobileTheme.colors.text,
    flex: 1,
    fontSize: 17,
    fontWeight: '600',
    lineHeight: 22,
    paddingHorizontal: mobileTheme.spacing.xs,
    textAlign: 'center',
  },
  sheetTitleStacked: {
    flex: 0,
  },
  scrollContent: {
    gap: mobileTheme.spacing.md,
    paddingBottom: mobileTheme.spacing.xxl,
  },
  groupedScrollContent: {
    gap: mobileTheme.spacing.lg,
    paddingTop: mobileTheme.spacing.sm,
  },
  stack: {
    gap: mobileTheme.spacing.md,
  },
  text: {
    color: mobileTheme.colors.text,
    ...mobileTheme.typography.body,
  },
  textMuted: {
    color: mobileTheme.colors.textMuted,
    ...mobileTheme.typography.body,
  },
});

