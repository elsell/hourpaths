import { useEffect, useRef, useState } from 'react';
import type { AdminAttentionPort, AdminAttentionNotice, AdminAttentionRecord, SupportPort, SupportConversation, SupportNotice } from '@hourpaths/client-core';
import { SupportFailure, adminReportReasonKey } from '@hourpaths/client-core';
import type { Translator } from '@hourpaths/i18n';
import { StyleSheet, View } from 'react-native';
import { NativeSheet, ActionButton, StatusBanner, ThemedText as Text, ThemedTextInput } from './primitives';
import { SettingsNavigationRow, SettingsSection } from './settings-list';
import { mobileTheme } from './tokens';

export function AdminAttentionView({support,moderation,i18n,newID,refreshing}:{support:SupportPort;moderation:AdminAttentionPort;i18n:Translator;newID:()=>string;refreshing:boolean}) {
 const [notices,setNotices]=useState<SupportNotice[]|null>(null),[reports,setReports]=useState<AdminAttentionNotice[]>([]),[inbox,setInbox]=useState<SupportConversation[]|null>(null),[cursor,setCursor]=useState(''),[ticket,setTicket]=useState<SupportConversation|null>(null),[record,setRecord]=useState<AdminAttentionRecord|null>(null),[busy,setBusy]=useState(false),[error,setError]=useState(false),[body,setBody]=useState('');
 const alive=useRef(true),pending=useRef<{id:string;body:string}|null>(null);
 useEffect(()=>{alive.current=true;return()=>{alive.current=false;};},[]);
 async function refresh(){try{const [s,m]=await Promise.all([support.notices(),moderation.notices()]);if(alive.current){setNotices(s);setReports(m);setError(false);}}catch(e){if(alive.current){if(e instanceof SupportFailure&&e.status===404){setNotices(null);setReports([]);setInbox(null);setTicket(null);setRecord(null);}else setError(true);}}}
 useEffect(()=>{if(!refreshing)void refresh();},[support,moderation,refreshing]);
 async function perform(action:()=>Promise<void>){setBusy(true);setError(false);try{await action();}catch{if(alive.current)setError(true);}finally{if(alive.current)setBusy(false);}}
 async function openTicket(id:string){await perform(async()=>{const c=await support.adminRead(id);if(alive.current){setTicket(c);setRecord(null);setBody('');pending.current=null;}await refresh();});}
 async function openInbox(after?:string){await perform(async()=>{const page=await support.inbox(after);if(alive.current){setInbox(old=>after?[...(old??[]),...page.items]:page.items);setCursor(page.nextCursor);}});}
 async function openRecord(id:string){await perform(async()=>{const r=await moderation.read(id);if(alive.current){setRecord(r);setTicket(null);}await refresh();});}
 async function reply(){if(!ticket)return;pending.current??={id:newID(),body};await perform(async()=>{const c=await support.adminReply(ticket.id,pending.current!.id,pending.current!.body);if(alive.current){setTicket(c);setBody('');pending.current=null;}});}
 const close=()=>{if(!busy){setInbox(null);setTicket(null);setRecord(null);setBody('');pending.current=null;}};
 if(notices===null&&!error)return null;
 return <View style={styles.section}>
  {error?<StatusBanner tone="error" text={i18n.t('support.failure')}/>:null}
  {notices!==null?<SettingsSection title={i18n.t('support.adminNotifications')}>
   <SettingsNavigationRow label={i18n.t('support.openInbox')} accessibilityLabel={i18n.t('support.openInbox')} onPress={()=>void openInbox()}/>
   {notices.map(n=><SettingsNavigationRow key={n.id} label={i18n.t('support.notice')} accessibilityLabel={i18n.t('support.notice')} value={i18n.date(new Date(n.createdAt),{dateStyle:'short'})} onPress={()=>void openTicket(n.requestId)}/>)}
   {reports.map(n=><SettingsNavigationRow key={n.id} label={i18n.t(n.kind==='report'?'adminAttention.report':'adminAttention.appeal')} accessibilityLabel={i18n.t(n.kind==='report'?'adminAttention.report':'adminAttention.appeal')} value={i18n.date(new Date(n.createdAt),{dateStyle:'short'})} onPress={()=>void openRecord(n.id)}/>)}
  </SettingsSection>:<ActionButton label={i18n.t('common.retry')} onPress={()=>void refresh()} variant="secondary"/>}
  <NativeSheet visible={inbox!==null||ticket!==null||record!==null} title={i18n.t(record?'adminAttention.heading':'support.inbox')} dismissible={!busy} onRequestClose={close} trailingAction={{label:i18n.t('common.done'),onPress:close,disabled:busy}}>
   {error?<StatusBanner tone="error" text={i18n.t('support.failure')}/>:null}
   {record?<View style={styles.section}><Text>{record.kind==='report'?i18n.t(adminReportReasonKey(record.reason)):record.reason}</Text><Text>{record.explanation}</Text><Text>{i18n.t('adminAttention.review')}</Text></View>:ticket?<View style={styles.section}>
    <Text accessibilityRole="header">{ticket.subject}</Text>
    {ticket.messages.map(m=><View key={m.id} style={styles.message}><Text style={styles.author}>{i18n.t(m.operator?'support.team':'support.requester')}</Text><Text>{m.body}</Text><Text>{i18n.date(new Date(m.createdAt),{dateStyle:'short',timeStyle:'short'})}</Text></View>)}
    <ActionButton label={i18n.t('support.refresh')} disabled={busy} onPress={()=>void perform(async()=>{const c=await support.adminRead(ticket.id);if(alive.current)setTicket(c);})} variant="secondary"/>
    {ticket.messages.length>=100?<Text>{i18n.t('support.messageLimit')}</Text>:<><Text>{i18n.t('support.message')}</Text><ThemedTextInput multiline accessibilityLabel={i18n.t('support.message')} value={body} editable={!busy&&pending.current===null} onChangeText={setBody} style={styles.input}/><ActionButton label={i18n.t('support.reply')} disabled={busy||!body.trim()||[...body].length>5000} onPress={()=>void reply()}/></>}
   </View>:<SettingsSection>{inbox?.length===0?<Text>{i18n.t('support.empty')}</Text>:null}{inbox?.map(c=><SettingsNavigationRow key={c.id} label={c.subject} accessibilityLabel={c.subject} onPress={()=>void openTicket(c.id)}/>)}{cursor?<ActionButton label={i18n.t('common.loadMore')} disabled={busy} onPress={()=>void openInbox(cursor)} variant="secondary"/>:null}</SettingsSection>}
  </NativeSheet>
 </View>;
}
const styles=StyleSheet.create({section:{gap:mobileTheme.spacing.md},message:{gap:mobileTheme.spacing.xs,padding:mobileTheme.spacing.md,borderWidth:StyleSheet.hairlineWidth,borderColor:mobileTheme.colors.border,borderRadius:12},author:{fontWeight:'600'},input:{minHeight:100,padding:mobileTheme.spacing.md,borderWidth:StyleSheet.hairlineWidth,borderColor:mobileTheme.colors.border,borderRadius:12}});
