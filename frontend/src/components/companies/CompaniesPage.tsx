import { useEffect, useRef, useState } from 'react'
import { api } from '../../services/api'

function Skeleton({rows=3}:{rows?:number}){
 return <div className="space-y-2 animate-pulse">{Array.from({length:rows}).map((_,i)=><div key={i} className="h-14 bg-zinc-100 rounded-xl" />)}</div>
}

function Field({label,value}:{label:string,value?:string}){
 return <div><div className="text-[10px] uppercase tracking-wide text-zinc-400">{label}</div><div className="text-sm">{value||<span className="text-zinc-300">—</span>}</div></div>
}

export function CompaniesPage(){
 const [q,setQ]=useState('')
 const [country,setCountry]=useState('')
 const [industry,setIndustry]=useState('')
 const [location,setLocation]=useState('')
 const [suggestions,setSuggestions]=useState<any[]>([])
 const [showSug,setShowSug]=useState(false)
 const [results,setResults]=useState<any[]|null>(null)
 const [notes,setNotes]=useState<string[]>([])
 const [searching,setSearching]=useState(false)
 const [profile,setProfile]=useState<any|null>(null)
 const [profileLoading,setProfileLoading]=useState(false)
 const [saved,setSaved]=useState<Record<string,boolean>>({})
 const [msg,setMsg]=useState('')
 const sugTimer=useRef<any>(null)

 useEffect(()=>{
  if(sugTimer.current) clearTimeout(sugTimer.current)
  if(q.trim().length<2){ setSuggestions([]); return }
  sugTimer.current=setTimeout(async()=>{
   try{ const r:any=await api.companies.suggest(q.trim()); setSuggestions(r.suggestions||[]) }catch{ setSuggestions([]) }
  },250)
  return ()=>{ if(sugTimer.current) clearTimeout(sugTimer.current) }
 },[q])

 const search=async()=>{
  if(!q.trim()) return
  setSearching(true); setShowSug(false); setMsg('')
  try{
   const r:any=await api.companies.search({q:q.trim(),country,industry,location})
   setResults(r.results||[]); setNotes(r.notes||[])
  }catch(e:any){ setResults([]); setMsg(e.message) }
  finally{ setSearching(false) }
 }

 const openProfile=async(r:any)=>{
  setProfileLoading(true); setProfile(null); setMsg('')
  try{ setProfile(await api.companies.profile({domain:r.domain||'',name:r.name||'',country})) }
  catch(e:any){ setMsg(e.message) }
  finally{ setProfileLoading(false) }
 }

 const saveToLeads=async(key:string,body:any)=>{
  try{
   const r:any=await api.companies.saveToLead(body)
   setSaved(s=>({...s,[key]:true}))
   setMsg(r.created?'Saved to leads':'Already in your leads')
  }catch(e:any){ setMsg(e.message) }
 }

 const exportCsv=()=>{ window.open(api.companies.exportUrl(),'_blank') }

 return (
  <div className="space-y-4">
   <div className="flex justify-between items-center">
    <h2 className="font-bold text-lg">Companies</h2>
    <button onClick={exportCsv} className="text-xs bg-white border px-3 py-2 rounded-xl hover:bg-zinc-50">Export to CSV</button>
   </div>

   <div className="bg-white border rounded-2xl p-4 space-y-3">
    <div className="relative">
     <input
      value={q}
      onChange={e=>{setQ(e.target.value);setShowSug(true)}}
      onFocus={()=>setShowSug(true)}
      onKeyDown={e=>{if(e.key==='Enter')search()}}
      placeholder="Company name, domain, or industry keywords..."
      className="w-full border rounded-xl px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-zinc-900"
     />
     {showSug && suggestions.length>0 && (
      <div className="absolute z-10 mt-1 w-full bg-white border rounded-xl shadow-lg overflow-hidden">
       {suggestions.map((s:any,i:number)=>(
        <button key={i} onClick={()=>{setQ(s.domain||s.name);setShowSug(false);openProfile(s)}} className="w-full text-left px-3 py-2 text-sm hover:bg-zinc-50 flex items-center gap-2">
         {s.logo && <img src={s.logo} alt="" className="w-4 h-4 rounded" />}
         <span className="font-medium">{s.name}</span>
         <span className="text-zinc-400 text-xs">{s.domain}</span>
        </button>
       ))}
      </div>
     )}
    </div>
    <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
     <select value={country} onChange={e=>setCountry(e.target.value)} className="border rounded-xl px-3 py-2 text-sm bg-white">
      <option value="">Any country</option>
      <option value="GB">United Kingdom</option>
      <option value="FR">France</option>
      <option value="RO">Romania</option>
     </select>
     <input value={industry} onChange={e=>setIndustry(e.target.value)} placeholder="Industry keywords" className="border rounded-xl px-3 py-2 text-sm" />
     <input value={location} onChange={e=>setLocation(e.target.value)} placeholder="City or country" className="border rounded-xl px-3 py-2 text-sm" />
     <button onClick={search} disabled={searching} className="bg-zinc-900 text-white px-4 py-2 rounded-xl text-sm disabled:opacity-50">{searching?'Searching...':'Search'}</button>
    </div>
   </div>

   {msg && <div className="text-xs bg-zinc-100 border rounded-xl px-3 py-2 text-zinc-600">{msg}</div>}

   {(profileLoading || profile) && (
    <div className="bg-white border rounded-2xl p-4 space-y-4">
     {profileLoading ? <Skeleton rows={4} /> : profile && (
      <>
       <div className="flex justify-between items-start gap-3">
        <div>
         <h3 className="font-semibold text-lg">{profile.name||profile.domain}</h3>
         {profile.domain && <a href={'https://'+profile.domain} target="_blank" rel="noreferrer" className="text-xs text-zinc-500 hover:text-zinc-900">{profile.domain}</a>}
        </div>
        <button onClick={()=>saveToLeads('profile',{name:profile.name,domain:profile.domain})} disabled={saved['profile']} className="bg-zinc-900 text-white px-4 py-2 rounded-xl text-sm disabled:opacity-50">{saved['profile']?'Saved ✓':'Save to Leads'}</button>
       </div>
       {profile.description && <p className="text-sm text-zinc-600">{profile.description}</p>}
       <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
        <Field label="Industry" value={profile.industry} />
        <Field label="Employees" value={profile.employee_count} />
        <Field label="Founded" value={profile.founded_year} />
        <Field label="HQ" value={profile.hq_location} />
        <Field label="Revenue (est.)" value={profile.revenue_estimate} />
        <Field label="Registry №" value={profile.registry_number} />
        <Field label="Registry status" value={profile.registry_status} />
        <Field label="Sources" value={(profile.sources||[]).join(', ')} />
       </div>
       {(profile.tech_stack||[]).length>0 && (
        <div><div className="text-[10px] uppercase tracking-wide text-zinc-400 mb-1">Tech stack</div>
         <div className="flex flex-wrap gap-1">{(profile.tech_stack||[]).map((t:string)=><span key={t} className="text-xs bg-zinc-100 rounded-full px-2 py-0.5">{t}</span>)}</div>
        </div>
       )}
       <div className="flex flex-wrap gap-2">
        {profile.linkedin_url && <a href={profile.linkedin_url} target="_blank" rel="noreferrer" className="text-xs border rounded-full px-3 py-1 hover:bg-zinc-50">LinkedIn</a>}
        {profile.twitter_url && <a href={profile.twitter_url} target="_blank" rel="noreferrer" className="text-xs border rounded-full px-3 py-1 hover:bg-zinc-50">Twitter / X</a>}
        {profile.facebook_url && <a href={profile.facebook_url} target="_blank" rel="noreferrer" className="text-xs border rounded-full px-3 py-1 hover:bg-zinc-50">Facebook</a>}
        {profile.crunchbase_url && <a href={profile.crunchbase_url} target="_blank" rel="noreferrer" className="text-xs border rounded-full px-3 py-1 hover:bg-zinc-50">Crunchbase</a>}
       </div>
       {(profile.notes||[]).length>0 && <div className="text-xs text-zinc-400 space-y-0.5">{(profile.notes||[]).map((n:string,i:number)=><div key={i}>• {n}</div>)}</div>}
      </>
     )}
    </div>
   )}

   <div className="bg-white border rounded-2xl p-4">
    {searching ? <Skeleton rows={5} /> : results===null ? (
     <div className="text-center py-8 text-zinc-400 text-sm">Search for a company to see firmographics, socials and tech stack.</div>
    ) : results.length===0 ? (
     <div className="text-center py-8 text-zinc-400 text-sm">No companies found. Try a different name or paste the exact domain.</div>
    ) : (
     <div className="divide-y">
      {results.map((r:any,i:number)=>(
       <div key={i} className="py-2.5 flex items-center justify-between gap-3">
        <button onClick={()=>openProfile(r)} className="text-left min-w-0">
         <div className="font-medium text-sm truncate">{r.name}</div>
         <div className="text-xs text-zinc-400 truncate">{[r.domain,r.status,r.address].filter(Boolean).join(' • ')}</div>
        </button>
        <div className="flex items-center gap-2 shrink-0">
         <span className="text-[10px] uppercase tracking-wide text-zinc-400">{r.source}</span>
         <button onClick={()=>saveToLeads(String(i),{name:r.name,domain:r.domain})} disabled={saved[String(i)]} className="text-xs border rounded-xl px-2.5 py-1.5 hover:bg-zinc-50 disabled:opacity-50">{saved[String(i)]?'Saved ✓':'Save'}</button>
        </div>
       </div>
      ))}
     </div>
    )}
    {notes.length>0 && <div className="mt-3 pt-3 border-t text-xs text-zinc-400 space-y-0.5">{notes.map((n,i)=><div key={i}>• {n}</div>)}</div>}
   </div>
  </div>
 )
}
