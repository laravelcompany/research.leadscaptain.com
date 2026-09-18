import { useEffect, useRef, useState } from 'react'
import { api } from '../../services/api'

function Card({title,desc,children}:{title:string,desc:string,children:any}){
 return (
  <div className="bg-white border rounded-2xl p-4 space-y-3">
   <div><h3 className="font-semibold text-sm">{title}</h3><p className="text-xs text-zinc-400">{desc}</p></div>
   {children}
  </div>
 )
}

function StatusBadge({status}:{status:string}){
 const c=status==='valid'?'bg-emerald-100 text-emerald-700':status==='risky'?'bg-amber-100 text-amber-700':status==='invalid'?'bg-red-100 text-red-700':'bg-zinc-100 text-zinc-600'
 return <span className={`text-xs font-medium rounded-full px-2 py-0.5 uppercase ${c}`}>{status}</span>
}

export function ToolsPage(){
 // Email verification
 const [email,setEmail]=useState('')
 const [emailResult,setEmailResult]=useState<any|null>(null)
 const [emailLoading,setEmailLoading]=useState(false)
 // Domain age
 const [domain,setDomain]=useState('')
 const [ageResult,setAgeResult]=useState<any|null>(null)
 const [ageLoading,setAgeLoading]=useState(false)
 // LinkedIn
 const [li,setLi]=useState('')
 const [liResult,setLiResult]=useState<any|null>(null)
 const [liCopied,setLiCopied]=useState(false)
 // Bulk
 const [bulkType,setBulkType]=useState('emails')
 const [bulkText,setBulkText]=useState('')
 const [bulkJob,setBulkJob]=useState<any|null>(null)
 const [bulkError,setBulkError]=useState('')
 const pollRef=useRef<any>(null)

 const verify=async()=>{
  if(!email.trim()) return
  setEmailLoading(true); setEmailResult(null)
  try{ setEmailResult(await api.tools.verifyEmail(email.trim())) }catch(e:any){ setEmailResult({error:e.message}) }
  finally{ setEmailLoading(false) }
 }
 const checkAge=async()=>{
  if(!domain.trim()) return
  setAgeLoading(true); setAgeResult(null)
  try{ setAgeResult(await api.tools.domainAge(domain.trim())) }catch(e:any){ setAgeResult({error:e.message}) }
  finally{ setAgeLoading(false) }
 }
 const formatLi=async()=>{
  if(!li.trim()) return
  setLiResult(null); setLiCopied(false)
  try{ setLiResult(await api.tools.linkedinFormat(li.trim())) }catch(e:any){ setLiResult({error:e.message}) }
 }
 const copyLi=()=>{ if(liResult?.formatted){ navigator.clipboard?.writeText(liResult.formatted); setLiCopied(true) } }

 const startBulk=async()=>{
  const items=bulkText.split(/[\n,;]+/).map(s=>s.trim()).filter(Boolean)
  setBulkError(''); setBulkJob(null)
  if(pollRef.current) clearInterval(pollRef.current)
  try{
   const r:any=await api.tools.bulkStart(bulkType,items)
   pollRef.current=setInterval(async()=>{
    try{
     const j:any=await api.tools.bulkStatus(r.id)
     setBulkJob(j)
     if(j.status==='completed' && pollRef.current){ clearInterval(pollRef.current); pollRef.current=null }
    }catch{}
   },800)
  }catch(e:any){ setBulkError(e.message) }
 }
 useEffect(()=>()=>{ if(pollRef.current) clearInterval(pollRef.current) },[])

 return (
  <div className="space-y-4">
   <h2 className="font-bold text-lg">Research Tools</h2>
   <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">

    <Card title="Email Verification" desc="Syntax, MX records and provider detection. No paid verification API.">
     <div className="flex gap-2">
      <input value={email} onChange={e=>setEmail(e.target.value)} onKeyDown={e=>{if(e.key==='Enter')verify()}} placeholder="name@company.com" className="flex-1 border rounded-xl px-3 py-2 text-sm" />
      <button onClick={verify} disabled={emailLoading} className="bg-zinc-900 text-white px-4 py-2 rounded-xl text-sm disabled:opacity-50">{emailLoading?'...':'Verify'}</button>
     </div>
     {emailResult && (
      emailResult.error ? <div className="text-xs text-red-600">{emailResult.error}</div> : (
       <div className="border rounded-xl p-3 space-y-1 text-sm">
        <div className="flex items-center gap-2"><StatusBadge status={emailResult.status} /><span className="text-xs text-zinc-500">{emailResult.provider}</span></div>
        <div className="text-xs text-zinc-500">{emailResult.reason}</div>
        {emailResult.mx_found && <div className="text-xs text-zinc-400">MX: {(emailResult.mx_hosts||[]).join(', ')}</div>}
       </div>
      )
     )}
    </Card>

    <Card title="Domain Age Checker" desc="Registration date, expiry and registrar via RDAP (free registry data).">
     <div className="flex gap-2">
      <input value={domain} onChange={e=>setDomain(e.target.value)} onKeyDown={e=>{if(e.key==='Enter')checkAge()}} placeholder="example.com" className="flex-1 border rounded-xl px-3 py-2 text-sm" />
      <button onClick={checkAge} disabled={ageLoading} className="bg-zinc-900 text-white px-4 py-2 rounded-xl text-sm disabled:opacity-50">{ageLoading?'...':'Check'}</button>
     </div>
     {ageResult && (
      ageResult.error ? <div className="text-xs text-red-600">{ageResult.error}</div> : (
       <div className="border rounded-xl p-3 grid grid-cols-2 gap-2 text-sm">
        <div><div className="text-[10px] uppercase text-zinc-400">Created</div>{ageResult.created||'—'}</div>
        <div><div className="text-[10px] uppercase text-zinc-400">Age</div>{ageResult.age_display||'—'}</div>
        <div><div className="text-[10px] uppercase text-zinc-400">Expires</div>{ageResult.expires||'—'}</div>
        <div><div className="text-[10px] uppercase text-zinc-400">Registrar</div>{ageResult.registrar||'—'}</div>
       </div>
      )
     )}
    </Card>

    <Card title="LinkedIn URL Formatter" desc="Standardize profile and company links for database entry.">
     <div className="flex gap-2">
      <input value={li} onChange={e=>setLi(e.target.value)} onKeyDown={e=>{if(e.key==='Enter')formatLi()}} placeholder="linkedin.com/in/jane-doe?utm_source=..." className="flex-1 border rounded-xl px-3 py-2 text-sm" />
      <button onClick={formatLi} className="bg-zinc-900 text-white px-4 py-2 rounded-xl text-sm">Format</button>
     </div>
     {liResult && (
      liResult.error ? <div className="text-xs text-red-600">{liResult.error}</div> : (
       <div className="border rounded-xl p-3 flex items-center justify-between gap-2">
        <div className="min-w-0">
         <div className="text-sm font-medium truncate">{liResult.formatted}</div>
         <div className="text-xs text-zinc-400">{liResult.kind} • {liResult.slug}</div>
        </div>
        <button onClick={copyLi} className="text-xs border rounded-xl px-3 py-1.5 hover:bg-zinc-50 shrink-0">{liCopied?'Copied ✓':'Copy'}</button>
       </div>
      )
     )}
    </Card>

    <Card title="Bulk Processing" desc="Paste up to 25 emails or URLs (one per line). Queued and processed in the background.">
     <div className="flex gap-2">
      <select value={bulkType} onChange={e=>setBulkType(e.target.value)} className="border rounded-xl px-3 py-2 text-sm bg-white">
       <option value="emails">Emails</option>
       <option value="urls">URLs</option>
      </select>
      <button onClick={startBulk} className="bg-zinc-900 text-white px-4 py-2 rounded-xl text-sm">Process</button>
     </div>
     <textarea value={bulkText} onChange={e=>setBulkText(e.target.value)} rows={4} placeholder={bulkType==='emails'?'jane@acme.com\njohn@globex.com':'acme.com\nglobex.com'} className="w-full border rounded-xl px-3 py-2 text-sm font-mono" />
     {bulkError && <div className="text-xs text-red-600">{bulkError}</div>}
     {bulkJob && (
      <div className="border rounded-xl p-3 space-y-2">
       <div className="flex items-center justify-between text-xs text-zinc-500">
        <span>Job #{bulkJob.id} • {bulkJob.status}</span>
        <span>{bulkJob.completed}/{bulkJob.total}</span>
       </div>
       <div className="h-2 bg-zinc-100 rounded-full overflow-hidden"><div className="h-full bg-zinc-700 transition-all" style={{width:(bulkJob.total?bulkJob.completed/bulkJob.total*100:0)+'%'}} /></div>
       <div className="max-h-48 overflow-auto divide-y">
        {(bulkJob.results||[]).map((r:any,i:number)=>(
         <div key={i} className="py-1.5 flex items-center justify-between gap-2 text-xs">
          <span className="truncate font-medium">{r.email||r.domain}</span>
          {r.email ? <StatusBadge status={r.status} /> : (
           <span className={r.reachable?'text-emerald-600':'text-red-500'}>{r.reachable?(r.title||'reachable'):'unreachable'}</span>
          )}
         </div>
        ))}
       </div>
      </div>
     )}
    </Card>

   </div>
  </div>
 )
}
