import { getLocales } from 'expo-localization';
import { router, useLocalSearchParams } from 'expo-router';
import { useEffect } from 'react';
import { createDeviceTranslator } from '../../../../../../src/i18n';
import { scheduleSocialRouteBootstrap } from '../../../../../../src/social-route-recovery';
import { useSocialInteractionRouteAncestry } from '../../../../../../src/use-social-interaction-route-ancestry';
import {
  dismissCommentHeartRosterRoute,
  recoverCommentHeartRosterRoute,
  useCommentHeartRosterRoutePresentation,
} from '../../../../../../src/ui/comment-heart-roster-route-presentation';
import { CommentHeartRosterView } from '../../../../../../src/ui/comment-heart-roster-view';
import { useSocialRouteRecovery } from '../../../../../../src/ui/social-route-recovery-presentation';
import { SocialRouteRecoveryView } from '../../../../../../src/ui/social-route-recovery-view';

const i18n = createDeviceTranslator(getLocales);

export default function CommentHeartRosterScreen() {
  const { commentID = '', eventID = '' } = useLocalSearchParams<{ commentID: string; eventID: string }>();
  const presentation = useCommentHeartRosterRoutePresentation(eventID, commentID);
  const target = { commentID, eventID, kind: 'comment-hearts' as const, routeKey: `social:comment-hearts:${eventID}:${commentID}` };
  const recovery = useSocialRouteRecovery(target);
  useSocialInteractionRouteAncestry(target);
  useEffect(() => () => dismissCommentHeartRosterRoute(), []);
  useEffect(() => { if (presentation === null) recoverCommentHeartRosterRoute(); }, [presentation]);
  useEffect(() => {
    if (presentation || recovery || !eventID || !commentID) return;
    return scheduleSocialRouteBootstrap(target, () => router.replace('/(tabs)/home'));
  }, [commentID, eventID, presentation, recovery]);
  if (!presentation) return <SocialRouteRecoveryView
    i18n={i18n}
    onGoFollowing={recovery?.onGoFollowing}
    onGoHome={recovery?.onGoHome}
    onRetry={recovery?.onRetry}
    state={recovery?.state ?? 'loading'}
  />;
  return <CommentHeartRosterView i18n={i18n} presentation={presentation} />;
}
