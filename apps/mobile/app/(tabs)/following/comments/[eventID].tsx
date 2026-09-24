import { getLocales } from 'expo-localization';
import { router, useLocalSearchParams, useNavigation } from 'expo-router';
import { useEffect, useRef } from 'react';
import { createDeviceTranslator } from '../../../../src/i18n';
import { scheduleSocialRouteBootstrap } from '../../../../src/social-route-recovery';
import {
  clearPracticeCommentRouteDrafts,
  practiceCommentRouteRemovalDecision,
} from '../../../../src/ui/comment-draft-presentation';
import { useSocialInteractionRouteAncestry } from '../../../../src/use-social-interaction-route-ancestry';
import {
  dismissPracticeCommentsRoute,
  recoverPracticeCommentsRoute,
  usePracticeCommentsRoutePresentation,
} from '../../../../src/ui/practice-comments-route-presentation';
import { PracticeCommentsView } from '../../../../src/ui/practice-comments-view';
import { presentNativeDestructiveConfirmation } from '../../../../src/ui/native-confirmation';
import { useSocialRouteRecovery } from '../../../../src/ui/social-route-recovery-presentation';
import { SocialRouteRecoveryView } from '../../../../src/ui/social-route-recovery-view';

const i18n = createDeviceTranslator(getLocales);

export default function PracticeCommentsScreen() {
  const { eventID = '' } = useLocalSearchParams<{ eventID: string }>();
  const presentation = usePracticeCommentsRoutePresentation(eventID);
  const navigation = useNavigation();
  const removalAllowed = useRef(false);
  const target = { eventID, kind: 'comments' as const, routeKey: `social:comments:${eventID}` };
  const recovery = useSocialRouteRecovery(target);
  useSocialInteractionRouteAncestry(target);
  useEffect(() => () => {
    dismissPracticeCommentsRoute();
    clearPracticeCommentRouteDrafts(eventID);
  }, [eventID]);
  useEffect(() => { if (presentation === null) recoverPracticeCommentsRoute(); }, [presentation]);
  useEffect(() => navigation.addListener('beforeRemove', (event) => {
    if (removalAllowed.current) return;
    const decision = practiceCommentRouteRemovalDecision(eventID, presentation?.busy ?? false);
    if (decision === 'allow') return;
    event.preventDefault();
    if (decision === 'block-busy') return;
    presentNativeDestructiveConfirmation({
      cancelLabel: i18n.t('common.cancel'),
      confirmLabel: i18n.t('social.commentsDiscardAction'),
      message: i18n.t('social.commentsDiscardDescription'),
      onConfirm: () => {
        removalAllowed.current = true;
        clearPracticeCommentRouteDrafts(eventID);
        navigation.dispatch(event.data.action);
      },
      title: i18n.t('social.commentsDiscardTitle'),
    });
  }), [eventID, navigation, presentation?.busy]);
  useEffect(() => {
    if (presentation || recovery || !eventID) return;
    return scheduleSocialRouteBootstrap(target, () => router.replace('/(tabs)/home'));
  }, [eventID, presentation, recovery]);
  if (!presentation) return <SocialRouteRecoveryView
    i18n={i18n}
    onGoFollowing={recovery?.onGoFollowing}
    onGoHome={recovery?.onGoHome}
    onRetry={recovery?.onRetry}
    state={recovery?.state ?? 'loading'}
  />;
  return <PracticeCommentsView i18n={i18n} presentation={presentation} />;
}
