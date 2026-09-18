import { useEffect, useState } from 'react'
import { api } from '../../services/api'

function Skeleton({rows=3}:{rows?:number}){
 return <div className="space-y-2 animate-pulse">{Array.from({length:rows}).map((_,i)=><div key={i} className="h-14 bg-zinc-100 rounded-xl" />)}</div>
}

function scoreColor(n:number){ return n>=80?'text-emerald-600':n>=50?'text-amber-600':'text-red-600' }
function scoreRing(n:number){ return n>=80?'border-emerald-500':n>=50?'border-amber-500':'border-red-500' }

function Report({a}:{a:any}){
 const contacts=a.contacts||{emails:a.emails,phones:a.phones,socials:a.socials}
 const checks=a.checks||[]
 const keywords=a.keywords||[]
 const emails=contacts.emails||a.emails||[]
 const phones=contacts.phones||a.phones||[]
 const socials=contacts.socials||a.socials||{}
 return (
  <div className="space-y-4">
   <div className="flex items-center gap-4 flex-wrap">
    <div className={`w-20 h-20 rounded-full border-4 ${scoreRing(a.health_score||0)} flex items-center justify-center`}>
     <div className={`text-2xl font-bold ${scoreColor(a.health_score||0)}`}>{a.health_score ?? '—'}</div>
    </div>
    <div className="min-w-0">
     <div className="font-semibold truncate">{a.title||a.domain||a.url}</div>
     <div className="text-xs text-zinc-400 truncate">{a.final_url||a.url} • {a.load_ms} ms{a.status_code?` • HTTP ${a.status_code}`:''}</div>
     {a.error && <div className="text-xs text-red-600">{a.error}</div>}
    </div>
   </div>

   {a.error ? null : <>
   <div>
    <div className="text-[10px] uppercase tracking-wide text-zinc-400 mb-1">SEO metrics</div>
    <div className="grid grid-cols-1 md:grid-cols-2 gap-2 text-sm">
     <div className="border rounded-xl p-3"><div className="text-xs text-zinc-400">Meta title</div>{a.title||<span className="text-zinc-300">missing</span>}</div>
     <div className="border rounded-xl p-3"><div className="text-xs text-zinc-400">Meta description</div>{a.meta_description||<span className="text-zinc-300">missing</span>}</div>
     <div className="border rounded-xl p-3"><div className="text-xs text-zinc-400">H1 tags</div>{(a.h1_tags||[]).length? (a.h1_tags||[]).join(', ') : <span className="text-zinc-300">none</span>}</div>
     <div className="border rounded-xl p-3"><div className="text-xs text-zinc-400">Page load</div>{a.load_ms} ms</div>
    </div>
   </div>

   <div>
    <div className="text-[10px] uppercase tracking-wide text-zinc-400 mb-1">Contact info found</div>
    {emails.length+phones.length===0 && !socials.linkedin && !socials.twitter && !socials.facebook ? (
     <div className="text-sm text-zinc-400">No emails, phones or social links found on the homepage or contact pages.</div>
    ) : (
     <div className="flex flex-wrap gap-2">
      {emails.map((e:string)=><a key={e} href={'mailto:'+e} className="text-xs border rounded-full px-3 py-1 hover:bg-zinc-50">{e}</a>)}
      {phones.map((p:string)=><a key={p} href={'tel:'+p} className="text-xs border rounded-full px-3 py-1 hover:bg-zinc-50">{p}</a>)}
      {socials.linkedin && <a href={socials.linkedin} target="_blank" rel="noreferrer" className="text-xs border rounded-full px-3 py-1 hover:bg-zinc-50">LinkedIn</a>}
      {socials.twitter && <a href={socials.twitter} target="_blank" rel="noreferrer" className="text-xs border rounded-full px-3 py-1 hover:bg-zinc-50">Twitter / X</a>}
      {socials.facebook && <a href={socials.facebook} target="_blank" rel="noreferrer" className="text-xs border rounded-full px-3 py-1 hover:bg-zinc-50">Facebook</a>}
     </div>
    )}
   </div>

   {keywords.length>0 && (
    <div>
     <div className="text-[10px] uppercase tracking-wide text-zinc-400 mb-1">Top keywords</div>
     <div className="space-y-1">
      {keywords.map((k:any)=>(
       <div key={k.word} className="flex items-center gap-2 text-xs">
        <span className="w-28 truncate font-medium">{k.word}</span>
        <div className="flex-1 h-2 bg-zinc-100 rounded-full overflow-hidden"><div className="h-full bg-zinc-700" style={{width:Math.min(100,k.density*10)+'%'}} /></div>
        <span className="w-20 text-right text-zinc-400">{k.count}x • {k.density.toFixed(1)}%</span>
       </div>
      ))}
     </div>
    </div>
   )}

   {checks.length>0 && (
    <div>
     <div className="text-[10px] uppercase tracking-wide text-zinc-400 mb-1">Health score breakdown</div>
     <div className="grid grid-cols-1 md:grid-cols-2 gap-1">
      {checks.map((c:any,i:number)=>(
       <div key={i} className="flex items-center gap-2 text-xs py-1">
        <span className={c.passed?'text-emerald-600':'text-red-500'}>{c.passed?'✓':'✗'}</span>
        <span className="flex-1">{c.name}{c.detail?` (${c.detail})`:''}</span>
        <span className="text-zinc-400">{c.points}/{c.max}</span>
       </div>
      ))}
     </div>
    </div>
   )}

   {a.traffic_note && <div className="text-xs text-zinc-400">Traffic insights: {a.traffic_note}</div>}
   </>}
  </div>
 )
}

export function WebsitesPage(){
 const [url,setUrl]=useState('')
 const [loading,setLoading]=useState(false)
 const [report,setReport]=useState<any|null>(null)
 const [error,setError]=useState('')
 const [history,setHistory]=useState<any[]>([])
 const [histLoading,setHistLoading]=useState(true)

 const loadHistory=async()=>{
  setHistLoading(true)
  try{ const r:any=await api.websites.list({per_page:'20'}); setHistory(r.data||[]) }catch{} finally{ setHistLoading(false) }
 }
 useEffect(()=>{ loadHistory() },[])

 const analyze=async(target?:string)=>{
  const u=(target||url).trim()
  if(!u) return
  setLoading(true); setError(''); setReport(null)
  try{
   const r:any=await api.websites.analyze(u)
   setReport(r.analysis||r)
   loadHistory()
  }catch(e:any){ setError(e.message) }
  finally{ setLoading(false) }
 }

 const openStored=async(id:number)=>{
  setLoading(true); setError('')
  try{
   const r:any=await api.websites.get(id)
   setReport({
    url:r.url, domain:r.domain, final_url:r.url, reachable:r.reachable, status_code:r.status_code,
    load_ms:r.load_ms, title:r.title, meta_description:r.meta_description, h1_tags:r.h1_tags||[],
    health_score:r.health_score, checks:r.checks||[], contacts:r.contacts||{}, keywords:r.keywords||[],
    traffic_note:r.traffic_note, error:r.error,
   })
  }catch(e:any){ setError(e.message) }
  finally{ setLoading(false) }
 }

 return (
  <div className="space-y-4">
   <h2 className="font-bold text-lg">Websites</h2>
   <div className="bg-white border rounded-2xl p-4">
    <div className="flex gap-2">
     <input value={url} onChange={e=>setUrl(e.target.value)} onKeyDown={e=>{if(e.key==='Enter')analyze()}}
      placeholder="example.com or https://example.com"
      className="flex-1 border rounded-xl px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-zinc-900" />
     <button onClick={()=>analyze()} disabled={loading} className="bg-zinc-900 text-white px-4 py-2 rounded-xl text-sm disabled:opacity-50">{loading?'Analyzing...':'Analyze'}</button>
    </div>
    <p className="text-xs text-zinc-400 mt-2">SEO metrics, page speed, contact scraping, keyword density and a 0-100 health score. No paid APIs - the site itself is the data source.</p>
   </div>

   {error && <div className="text-xs bg-red-50 border border-red-200 rounded-xl px-3 py-2 text-red-700">{error}</div>}

   {(loading || report) && (
    <div className="bg-white border rounded-2xl p-4">
     {loading ? <Skeleton rows={5} /> : report && <Report a={report} />}
    </div>
   )}

   <div className="bg-white border rounded-2xl p-4">
    <h3 className="font-semibold mb-2 text-sm">Recent analyses</h3>
    {histLoading ? <Skeleton rows={3} /> : history.length===0 ? (
     <div className="text-center py-6 text-zinc-400 text-sm">No websites analyzed yet.</div>
    ) : (
     <div className="divide-y">
      {history.map((h:any)=>(
       <button key={h.id} onClick={()=>openStored(h.id)} className="w-full py-2.5 flex items-center justify-between gap-3 text-left hover:bg-zinc-50 -mx-2 px-2 rounded-xl">
        <div className="min-w-0">
         <div className="font-medium text-sm truncate">{h.title||h.domain}</div>
         <div className="text-xs text-zinc-400 truncate">{h.domain} • {h.load_ms} ms • {h.created_at}</div>
        </div>
        <span className={`text-sm font-bold shrink-0 ${scoreColor(h.health_score||0)}`}>{h.health_score ?? '—'}</span>
       </button>
      ))}
     </div>
    )}
   </div>
  </div>
 )
}
