import { useState } from 'react'
import { Badge } from '../ui/Badge'
import { Drawer } from '../ui/Drawer'
import { EmptyState } from '../ui/EmptyState'
function Score({v}:{v:number}){
 const pct=Math.max(0,Math.min(100,v))
 const variant=pct>=80?'success':pct>=50?'warning':'danger'
 const label=pct>=80?'High':pct>=50?'Medium':'Low'
 return <span className="inline-flex items-center gap-2"><span className="w-16 h-1.5 bg-zinc-200 rounded-full overflow-hidden inline-block"><span className={`block h-full ${pct>=80?'bg-emerald-500':pct>=50?'bg-amber-500':'bg-zinc-400'}`} style={{width:pct+'%'}}/></span><Badge variant={variant as any}>{label} {pct}</Badge></span>
}
export function LeadTable({leads,loading,search,setSearch,onSearch}:{leads:any[],loading:boolean,search:string,setSearch:(v:string)=>void,onSearch:()=>void}){
 const [sel,setSel]=useState<any|null>(null)
 const [page,setPage]=useState(1)
 const [country,setCountry]=useState('')
 const [sortBy,setSortBy]=useState<'name'|'company'|'country'|'score'>('name')
 const [sortDir,setSortDir]=useState<'asc'|'desc'>('asc')
 const per=25
 let filtered=Array.isArray(leads)?leads:[]
 if(country) filtered=filtered.filter((l:any)=>(l.country_code||l.country||'').toUpperCase()===country.toUpperCase())
 filtered=[...filtered].sort((a:any,b:any)=>{
  let va='',vb=''
  if(sortBy==='country'){va=a.country_code||'';vb=b.country_code||''}
  else if(sortBy==='company'){va=a.company_name||'';vb=b.company_name||''}
  else if(sortBy==='score'){return sortDir==='asc'?a.lead_score-b.lead_score:b.lead_score-a.lead_score}
  else {va=(a.first_name+' '+a.last_name).toLowerCase();vb=(b.first_name+' '+b.last_name).toLowerCase()}
  const c=va.localeCompare(vb); return sortDir==='asc'?c:-c
 })
 const countries=[...new Set((Array.isArray(leads)?leads:[]).map((l:any)=>l.country_code).filter(Boolean))].sort() as string[]
 const total=filtered.length
 const slice=filtered.slice((page-1)*per, page*per)
 return (
  <div className="space-y-3">
    <div className="flex flex-wrap gap-2 items-center bg-zinc-50 border rounded-xl p-2">
     <input value={search} onChange={e=>setSearch(e.target.value)} onKeyDown={e=>e.key==='Enter'&&onSearch()} placeholder="Search name, email, company..." className="flex-1 min-w-[200px] border rounded-lg px-3 py-2 text-sm bg-white"/>
     <select value={country} onChange={e=>{setCountry(e.target.value); setPage(1)}} className="border rounded-lg px-3 py-2 text-sm bg-white"><option value="">All countries</option>{countries.map(c=><option key={c} value={c}>{c}</option>)}</select>
     <select value={sortBy} onChange={e=>setSortBy(e.target.value as any)} className="border rounded-lg px-3 py-2 text-sm bg-white"><option value="name">Sort: Name</option><option value="country">Sort: Country</option><option value="company">Sort: Company</option><option value="score">Sort: Score</option></select>
     <button onClick={()=>setSortDir(d=>d==='asc'?'desc':'asc')} className="border bg-white px-3 py-2 rounded-lg text-sm">{sortDir==='asc'?'↑':'↓'}</button>
     <button onClick={onSearch} className="bg-zinc-900 text-white px-4 py-2 rounded-lg text-sm">Search</button>
     <button onClick={async()=>{if(!confirm('Clear ALL leads from database?'))return; await import('../../services/api').then(m=>m.api.clearLeads()); setSearch(''); setCountry(''); onSearch()}} className="border bg-white hover:bg-red-50 text-red-600 px-3 py-2 rounded-lg text-sm">Clear DB</button>
     <button onClick={()=>{setSearch(''); setCountry(''); onSearch()}} className="border bg-white px-3 py-2 rounded-lg text-sm">Clear filter</button>
     <a href="/api/v1/leads/export" className="ml-auto bg-white border px-3 py-2 rounded-lg text-sm">Export CSV</a>
    </div>
   <div className="bg-white border rounded-xl overflow-hidden">
    <div className="overflow-auto max-h-[60vh]">
     <table className="w-full text-sm">
        <thead className="sticky top-0 bg-zinc-50 border-b"><tr><th className="text-left p-3 font-semibold">Name</th><th className="text-left p-3 font-semibold hidden md:table-cell">Company</th><th className="text-left p-3 font-semibold">Title</th><th className="text-left p-3 font-semibold hidden sm:table-cell">Email</th><th className="text-left p-3 font-semibold hidden lg:table-cell">Country</th><th className="text-left p-3 font-semibold hidden lg:table-cell">LinkedIn</th><th className="text-left p-3 font-semibold">Score</th></tr></thead>
       <tbody>
        {loading ? <tr><td colSpan={7} className="p-8 text-center text-zinc-400">Loading...</td></tr> :
         slice.map((l:any)=><tr key={l.id} onClick={()=>setSel(l)} className="border-t hover:bg-zinc-50 cursor-pointer">
          <td className="p-3"><div className="font-medium">{l.first_name} {l.last_name}</div><div className="text-xs text-zinc-500 md:hidden">{l.company_name}</div></td>
          <td className="p-3 hidden md:table-cell">{l.company_name}</td>
          <td className="p-3"><span className="bg-blue-50 text-blue-700 px-2 py-1 rounded-full text-xs">{l.position_title}</span></td>
          <td className="p-3 hidden sm:table-cell font-mono text-xs"><div>{l.email}</div>{l.email_status && <span className={`text-[10px] px-1 rounded ${l.email_status==='valid'?'bg-emerald-100 text-emerald-700':l.email_status==='invalid'?'bg-red-100 text-red-700':'bg-zinc-100 text-zinc-500'}`}>{l.email_status}</span>}</td>
          <td className="p-3 hidden lg:table-cell"><span className="bg-zinc-100 px-2 py-1 rounded-full text-xs">{l.country_code || l.country || '-'}</span></td>
          <td className="p-3 hidden lg:table-cell">{l.linkedin_url ? <a href={l.linkedin_url} target="_blank" rel="noreferrer" onClick={e=>e.stopPropagation()} className="text-blue-600 hover:underline text-xs">LinkedIn ↗</a> : <span className="text-zinc-400 text-xs">-</span>}</td>
          <td className="p-3"><Score v={l.lead_score}/></td>
         </tr>)}
      </tbody>
     </table>
     {!loading && (Array.isArray(slice)?slice:[]).length===0 && <EmptyState title="No leads found" desc="Try adjusting search or start an objective." />}
    </div>
    <div className="flex items-center justify-between p-3 border-t bg-zinc-50 text-xs">
     <span>Showing {(page-1)*per+1}–{Math.min(page*per,total)} of {total}</span>
     <div className="flex gap-1"><button disabled={page<=1} onClick={()=>setPage(p=>p-1)} className="border bg-white px-2 py-1 rounded disabled:opacity-50">Prev</button><span className="px-2 py-1">Page {page}</span><button disabled={page*per>=total} onClick={()=>setPage(p=>p+1)} className="border bg-white px-2 py-1 rounded disabled:opacity-50">Next</button></div>
    </div>
   </div>
   <Drawer open={!!sel} onClose={()=>setSel(null)} title="Lead Details">
    {sel && (
     <div className="space-y-4">
       <div><div className="text-lg font-bold">{sel.first_name} {sel.last_name}</div><div className="text-sm text-zinc-500">{sel.position_title} · {sel.company_name}</div></div>
       <Score v={sel.lead_score}/>
        <div className="space-y-2 text-sm"><div><span className="text-zinc-500">Email {sel.email_status && <span className={`ml-2 text-xs px-1 rounded ${sel.email_status==='valid'?'bg-emerald-100 text-emerald-700':sel.email_status==='invalid'?'bg-red-100 text-red-700':'bg-zinc-100'}`}>{sel.email_status}</span>}</span><div className="font-mono bg-zinc-50 border rounded-lg p-2">{sel.email}</div></div><div><span className="text-zinc-500">Company</span><div className="font-medium">{sel.company_name} {sel.company_domain && `(${sel.company_domain})`}</div></div><div><span className="text-zinc-500">Location</span><div>{[sel.city, sel.country_code].filter(Boolean).join(', ') || '-'}</div></div><div><span className="text-zinc-500">Industry</span><div>{sel.industry_name || '-'}</div></div>{sel.linkedin_url && <div><span className="text-zinc-500">LinkedIn</span><div><a href={sel.linkedin_url} target="_blank" rel="noreferrer" className="text-blue-600 hover:underline break-all">{sel.linkedin_url}</a></div></div>}{sel.summary && <div><span className="text-zinc-500">Summary</span><div className="bg-zinc-50 border rounded-lg p-2 text-xs whitespace-pre-wrap">{sel.summary}</div></div>}<div className="bg-blue-50 border border-blue-200 rounded-xl p-3"><div className="text-xs font-bold text-blue-700 mb-1">LinkedIn Intro {sel.outreach_message ? '' : '(generating...)'}</div><div className="text-sm whitespace-pre-wrap">{sel.outreach_message || 'Generating personalized message (2 at a time after email check)...'}</div>{sel.outreach_message && <button onClick={()=>navigator.clipboard.writeText(sel.outreach_message)} className="mt-2 border bg-white px-3 py-1 rounded-lg text-xs">Copy</button>}</div><div><span className="text-zinc-500">Source</span><div>LeadsCaptain</div></div></div>
     </div>
    )}
   </Drawer>
  </div>
 )
}
