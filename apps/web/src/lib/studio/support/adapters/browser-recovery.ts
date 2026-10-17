import { supportTicketID } from "@hourpaths/client-core";
// Browser capability: recovery-fragment capture, tab-history state and random key.
export function browserSupportRecovery(){
 const params=new URLSearchParams(window.location.hash.slice(1));
 const retained=history.state?.hourpathsSupport;
 const saved=retained&&typeof retained.id==='string'&&typeof retained.key==='string'?retained:undefined;
 const id=params.get('ticket')??saved?.id,key=params.get('key')??saved?.key;
 const existing=Boolean(id&&key);
 const secret=key??[...crypto.getRandomValues(new Uint8Array(32))].map(x=>x.toString(16).padStart(2,'0')).join('');
 const requestId=id??supportTicketID(Date.now(),[...crypto.getRandomValues(new Uint8Array(10))]),link=new URL('/support',window.location.origin);
 link.hash=new URLSearchParams({ticket:requestId,key:secret}).toString();
 const remember=()=>history.replaceState({hourpathsSupport:{id:requestId,key:secret}},'',window.location.pathname);
 if(existing)remember();
 else if(window.location.hash)history.replaceState(null,'',window.location.pathname);
 return {id:requestId,key:secret,link:link.toString(),existing,remember};
}
