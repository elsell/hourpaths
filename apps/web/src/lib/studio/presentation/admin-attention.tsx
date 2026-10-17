import { useEffect, useState } from 'react';
import type { Translator } from '@hourpaths/i18n';
import type { AdminAttentionPort, AdminAttentionNotice, AdminAttentionRecord } from '@hourpaths/client-core';
import { SupportFailure, adminReportReasonKey } from '@hourpaths/client-core';
export function AdminAttention({service,i18n}:{service:AdminAttentionPort;i18n:Translator}) {
 const [items,setItems]=useState<AdminAttentionNotice[]|null>(null),[record,setRecord]=useState<AdminAttentionRecord|null>(null),[error,setError]=useState(false),[busy,setBusy]=useState(false);
 useEffect(()=>{let active=true;void service.notices().then(v=>{if(active)setItems(v);}).catch(e=>{if(active&&!(e instanceof SupportFailure&&e.status===404))setError(true);});return()=>{active=false;};},[service]);
 async function open(id:string){setBusy(true);setError(false);try{setRecord(await service.read(id));setItems(await service.notices());}catch{setError(true);}finally{setBusy(false);}}
 if(items===null&&!error)return null;
 return <section className="studio-card"><h2>{i18n.t('adminAttention.heading')}</h2>{error&&<p role="alert">{i18n.t('support.failure')}</p>}{record?<><button onClick={()=>setRecord(null)}>{i18n.t('common.back')}</button><h3>{i18n.t(record.kind==='report'?'adminAttention.report':'adminAttention.appeal')}</h3><p>{i18n.date(new Date(record.createdAt),{dateStyle:'medium',timeStyle:'short'})}</p><dl><dt>{i18n.t('adminAttention.reason')}</dt><dd>{record.kind==='report'?i18n.t(adminReportReasonKey(record.reason)):record.reason}</dd><dt>{i18n.t('adminAttention.explanation')}</dt><dd style={{whiteSpace:'pre-wrap',overflowWrap:'anywhere'}}>{record.explanation}</dd></dl><p>{i18n.t('adminAttention.review')}</p></>:<ul>{items?.map(n=><li key={n.id}><button disabled={busy} onClick={()=>void open(n.id)}><strong>{i18n.t(n.kind==='report'?'adminAttention.report':'adminAttention.appeal')}</strong> · {i18n.date(new Date(n.createdAt),{dateStyle:'medium',timeStyle:'short'})}</button></li>)}</ul>}</section>;
}
