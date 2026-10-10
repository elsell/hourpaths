import { PathEmoji } from './path-emoji';
import { getLocales } from 'expo-localization';
import { Fragment, type ReactNode } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';
import { createDeviceTranslator } from '../i18n';
import { pathColorKeys, pathPalette, type PathAppearance, type PathColor } from './path-appearance';
import { PlatformSymbol } from './platform-symbol';
import { SectionHeading, StatusBanner, Surface } from './primitives';
import { SettingsIcon } from './settings-icon';
import { SettingsNavigationRow, SettingsSection, SettingsSeparator } from './settings-list';
import { mobileTheme } from './tokens';

const i18n = createDeviceTranslator(getLocales);

export type PathDetailViewProps = {
  actionDisabled?: boolean;
  comparisonFirst?: boolean;
  appearance: PathAppearance;
  name: string;
  onRename?: () => void;
  onAppearance?: () => void;
  onColorChange?: (color: PathColor) => void;
  appearanceBusy?: boolean;
  headline?: string;
  periodLabel?: string;
  goalText?: string;
  progress?: ReactNode;
  accumulatedText?: string;
  timer?: ReactNode;
  recentActivity?: ReactNode;
  statistics?: ReactNode;
  settings: readonly {
    label: string;
    value?: string;
    onPress: () => void;
    disabled?: boolean;
    systemImage?: string;
  }[];
  participantComparison?: ReactNode;
  archived: boolean;
  busy: boolean;
};

export function PathDetailView({
  actionDisabled = false, comparisonFirst = false, appearance, name, onRename, onAppearance, onColorChange, appearanceBusy = false,
  headline, periodLabel, goalText, progress, accumulatedText, timer, recentActivity, statistics,
  settings, participantComparison, archived, busy,
}: PathDetailViewProps) {
  const tone = pathPalette[appearance.color];
  const canRename = Boolean(onRename) && !archived;
  const canChangeAppearance = Boolean(onAppearance) && !archived;
  const comparison = participantComparison ? <View style={styles.section}>
    <SectionHeading>{i18n.t('pathMembers.heading')}</SectionHeading>
    {participantComparison}
  </View> : null;
  return <View style={styles.stack}>
    {comparisonFirst ? comparison : null}
    {archived ? <Surface>
      <Text accessibilityRole="alert" style={styles.archived}>{i18n.t('pathArchive.readOnly')}</Text>
    </Surface> : null}

    <View style={[styles.hero, { backgroundColor: tone.background }]}>
      <View style={styles.identity}>
        <Pressable
          accessibilityLabel={i18n.t('home.appearance.emoji')}
          accessibilityRole={canChangeAppearance ? 'button' : undefined}
          accessibilityState={{ disabled: appearanceBusy }}
          disabled={!canChangeAppearance || appearanceBusy || actionDisabled}
          onPress={onAppearance}
          style={({ pressed }) => [styles.emojiButton, pressed && styles.pressed]}
        >
          <PathEmoji emoji={appearance.emoji} size={48} />
        </Pressable>
        <Pressable
          accessibilityLabel={name}
          accessibilityRole={canRename ? 'button' : 'header'}
          disabled={!canRename || actionDisabled}
          onPress={onRename}
          style={({ pressed }) => [styles.nameButton, pressed && styles.pressed]}
        >
          <Text style={[styles.name, { color: tone.foreground }]}>{name}</Text>
          {canRename ? <PlatformSymbol systemName="pencil" color={tone.accent} size={mobileTheme.spacing.md} /> : null}
        </Pressable>
      </View>

      {onColorChange && !archived ? <View accessibilityLabel={i18n.t('home.appearance.color')} style={styles.palette}>
        {pathColorKeys.map((color) => <Pressable
          key={color}
          accessibilityLabel={i18n.t(`home.appearance.color.${color}`)}
          accessibilityRole="button"
          accessibilityState={{ selected: color === appearance.color, disabled: appearanceBusy }}
          disabled={appearanceBusy || actionDisabled}
          onPress={() => onColorChange(color)}
          style={({ pressed }) => [styles.swatchButton, pressed && styles.pressed, appearanceBusy && styles.disabled]}
        >
          <View style={[styles.swatch, { backgroundColor: pathPalette[color].background, borderColor: pathPalette[color].accent }]}>
            {color === appearance.color ? <PlatformSymbol systemName="checkmark" color={pathPalette[color].accent} size={mobileTheme.spacing.md} /> : null}
          </View>
        </Pressable>)}
      </View> : null}

      {onColorChange && !archived ? <Text style={[styles.caption, { color: tone.foreground }]}>{i18n.t('pathDetails.appearancePersonal')}</Text> : null}
      {headline ? <View style={styles.metric}>
        <Text style={[styles.total, { color: tone.foreground }]}>{headline}</Text>
        {periodLabel ? <Text style={[styles.caption, { color: tone.foreground }]}>{periodLabel}</Text> : null}
      </View> : null}
      {goalText ? <Text style={[styles.caption, { color: tone.foreground }]}>{goalText}</Text> : null}
      {progress}
      {accumulatedText ? <Text style={[styles.caption, { color: tone.foreground }]}>{accumulatedText}</Text> : null}
      {!archived ? timer : null}
    </View>

    {statistics}
    {recentActivity ? <View style={styles.section}>
      <SectionHeading>{i18n.t('pathDetails.recentActivity')}</SectionHeading>
      {recentActivity}
    </View> : null}

    {settings.length ? <SettingsSection title={i18n.t('pathDetails.settings')}>
      {settings.map((setting, index) => <Fragment key={setting.label}>
        {index > 0 ? <SettingsSeparator /> : null}
        <SettingsNavigationRow
          accessibilityLabel={setting.label}
          disabled={setting.disabled || actionDisabled}
          icon={setting.systemImage ? <SettingsIcon systemName={setting.systemImage} /> : undefined}
          label={setting.label}
          onPress={setting.onPress}
          value={setting.value}
        />
      </Fragment>)}
    </SettingsSection> : null}

    {!comparisonFirst ? comparison : null}
    {busy ? <StatusBanner text={i18n.t('common.loading')} /> : null}
  </View>;
}

const styles = StyleSheet.create({
  archived: { ...mobileTheme.typography.body, color: mobileTheme.colors.textMuted },
  stack: { gap: mobileTheme.spacing.lg },
  section: { gap: mobileTheme.spacing.sm },
  hero: { borderRadius: mobileTheme.radii.lg, borderCurve: 'continuous', padding: mobileTheme.spacing.md, gap: mobileTheme.spacing.xxs },
  identity: { flexDirection: 'row', alignItems: 'center', gap: mobileTheme.spacing.xs },
  emojiButton: { minHeight: mobileTheme.sizes.minimumTouchTarget, minWidth: mobileTheme.sizes.minimumTouchTarget, alignItems: 'center', justifyContent: 'center' },
  emoji: { ...mobileTheme.typography.heading },
  nameButton: { flex: 1, flexDirection: 'row', alignItems: 'center', gap: mobileTheme.spacing.xs, minHeight: mobileTheme.sizes.minimumTouchTarget },
  name: { ...mobileTheme.typography.subheading, flexShrink: 1 },
  palette: { flexDirection: 'row', flexWrap: 'wrap' },
  swatchButton: { minHeight: mobileTheme.sizes.minimumTouchTarget, minWidth: mobileTheme.sizes.minimumTouchTarget, alignItems: 'center', justifyContent: 'center' },
  swatch: { width: mobileTheme.spacing.lg, height: mobileTheme.spacing.lg, borderRadius: mobileTheme.radii.pill, borderWidth: mobileTheme.sizes.border, alignItems: 'center', justifyContent: 'center' },
  metric: { gap: mobileTheme.spacing.xxs },
  total: { ...mobileTheme.typography.title, fontVariant: ['tabular-nums'] },
  caption: { ...mobileTheme.typography.caption },
  pressed: { opacity: 0.65 },
  disabled: { opacity: 0.48 },
});
