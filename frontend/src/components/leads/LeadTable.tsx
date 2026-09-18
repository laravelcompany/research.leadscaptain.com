import { useState } from 'react'
import { Badge } from '../ui/Badge'
import { Drawer } from '../ui/Drawer'
import { EmptyState } from '../ui/EmptyState'

type SortKey='name'|'company'|'title'|'email'|'country'|'linkedin'|'score'
type SortDirection='asc'|'desc'

const priorityCountries=[
 ['US','United States'],['GB','United Kingdom'],['CA','Canada'],['DE','Germany'],['FR','France'],
 ['IT','Italy'],['ES','Spain'],['NL','Netherlands'],['CH','Switzerland'],['SE','Sweden'],
 ['NO','Norway'],['DK','Denmark'],['BE','Belgium'],['AT','Austria'],['IE','Ireland'],
 ['AU','Australia'],['NZ','New Zealand'],['JP','Japan'],['KR','South Korea'],['CN','China'],
 ['IN','India'],['SG','Singapore'],['AE','United Arab Emirates'],['SA','Saudi Arabia'],['BR','Brazil'],
 ['MX','Mexico'],['ZA','South Africa'],['IL','Israel'],['FI','Finland'],['PL','Poland'],
 ['LU','Luxembourg'],['PT','Portugal'],['CZ','Czech Republic'],['EE','Estonia'],['RO','Romania'],
] as const

const sortValue=(lead:any,key:Exclude<SortKey,'score'>)=>{
 if(key==='name') return lead.full_name||`${lead.first_name||''} ${lead.last_name||''}`.trim()
 if(key==='company') return lead.company_name||''
 if(key==='title') return lead.position_title||''
 if(key==='email') return lead.email||''
 if(key==='country') return lead.country_name||lead.country_code||lead.country||''
 return lead.linkedin_url||''
}
function Score({v}:{v:number}){
 const pct=Math.max(0,Math.min(100,v))
 const variant=pct>=80?'success':pct>=50?'warning':'danger'
 const label=pct>=80?'High':pct>=50?'Medium':'Low'
 return <span className="inline-flex items-center gap-2"><span className="w-16 h-1.5 bg-zinc-200 rounded-full overflow-hidden inline-block"><span className={`block h-full ${pct>=80?'bg-emerald-500':pct>=50?'bg-amber-500':'bg-zinc-400'}`} style={{width:pct+'%'}}/></span><Badge variant={variant as any}>{label} {pct}</Badge></span>
}
function StatusChip({v,site}:{v?:string,site?:boolean}){
 if(!v) return null
 const cls=(v==='valid'||v==='live')?'bg-emerald-100 text-emerald-700':(v==='invalid'||v==='unreachable'||v==='parked')?'bg-red-100 text-red-700':(v==='risky'||v==='error')?'bg-amber-100 text-amber-700':'bg-zinc-100 text-zinc-500'
 return <span className={`text-[10px] px-1 rounded ${cls}`}>{site?'site: ':''}{v}</span>
}
export function LeadTable({leads,loading,search,setSearch,onSearch}:{leads:any[],loading:boolean,search:string,setSearch:(v:string)=>void,onSearch:()=>void}){
 const [sel,setSel]=useState<any|null>(null)
 const [page,setPage]=useState(1)
 const [country,setCountry]=useState('')
 const [sortBy,setSortBy]=useState<SortKey>('name')
 const [sortDir,setSortDir]=useState<SortDirection>('asc')
 const [scoreBand,setScoreBand]=useState('')
 const per=25
 let filtered=Array.isArray(leads)?leads:[]
 if(country) filtered=filtered.filter((l:any)=>(l.country_code||l.country||'').toUpperCase()===country.toUpperCase())
 if(scoreBand) filtered=filtered.filter((l:any)=>{
  const v=l.lead_score??0
  return scoreBand==='high'?v>=80:scoreBand==='medium'?v>=50&&v<80:v<50
 })
 filtered=[...filtered].sort((a:any,b:any)=>{
  const comparison=sortBy==='score'
   ? Number(a.lead_score||0)-Number(b.lead_score||0)
   : sortValue(a,sortBy).localeCompare(sortValue(b,sortBy),undefined,{sensitivity:'base'})
  return sortDir==='asc'?comparison:-comparison
 })
 const knownCountryCodes=new Set(priorityCountries.map(([code])=>code))
 const extraCountries=[...new Set((Array.isArray(leads)?leads:[]).map((l:any)=>(l.country_code||l.country||'').toUpperCase()).filter((code:string)=>code&&!knownCountryCodes.has(code as any)))].sort() as string[]
 const toggleSort=(key:SortKey)=>{
  if(sortBy===key)setSortDir(direction=>direction==='asc'?'desc':'asc')
  else {setSortBy(key);setSortDir('asc')}
  setPage(1)
 }
 const SortHeader=({column,label,className=''}:{column:SortKey,label:string,className?:string})=><th aria-sort={sortBy===column?(sortDir==='asc'?'ascending':'descending'):'none'} className={`text-left p-0 font-semibold ${className}`}><button type="button" onClick={()=>toggleSort(column)} className="w-full p-3 text-left hover:bg-zinc-100 focus:outline-none focus:ring-2 focus:ring-inset focus:ring-zinc-500 whitespace-nowrap">{label} <span aria-hidden="true" className={sortBy===column?'text-zinc-900':'text-zinc-300'}>{sortBy===column?(sortDir==='asc'?'↑':'↓'):'↕'}</span></button></th>
 const total=filtered.length
 const pages=Math.max(1,Math.ceil(total/per))
 const curPage=Math.min(page,pages)
 const slice=filtered.slice((curPage-1)*per, curPage*per)
 return (
  <div className="space-y-3">
    <div className="flex flex-wrap gap-2 items-center bg-zinc-50 border rounded-xl p-2">
     <input value={search} onChange={e=>{setSearch(e.target.value); setPage(1)}} onKeyDown={e=>e.key==='Enter'&&onSearch()} placeholder="Search name, email, company..." className="flex-1 min-w-[200px] border rounded-lg px-3 py-2 text-sm bg-white"/>
     <select aria-label="Filter by country" value={country} onChange={e=>{setCountry(e.target.value); setPage(1)}} className="border rounded-lg px-3 py-2 text-sm bg-white"><option value="">All countries</option>{priorityCountries.map(([code,name])=><option key={code} value={code}>{name} ({code})</option>)}{extraCountries.length>0&&<optgroup label="Other countries in results">{extraCountries.map(code=><option key={code} value={code}>{code}</option>)}</optgroup>}</select>
     <select value={scoreBand} onChange={e=>{setScoreBand(e.target.value); setPage(1)}} className="border rounded-lg px-3 py-2 text-sm bg-white"><option value="">All scores</option><option value="high">High (80+)</option><option value="medium">Medium (50-79)</option><option value="low">Low (&lt;50)</option></select>
     <button onClick={onSearch} className="bg-zinc-900 text-white px-4 py-2 rounded-lg text-sm">Search</button>
     <button onClick={async()=>{if(!confirm('Clear ALL leads from database?'))return; await import('../../services/api').then(m=>m.api.clearLeads()); setSearch(''); setCountry(''); setScoreBand(''); setPage(1); onSearch()}} className="border bg-white hover:bg-red-50 text-red-600 px-3 py-2 rounded-lg text-sm">Clear DB</button>
     <button onClick={()=>{setSearch(''); setCountry(''); setScoreBand(''); setPage(1); onSearch()}} className="border bg-white px-3 py-2 rounded-lg text-sm">Clear filter</button>
     <a href="/api/v1/leads/export" className="ml-auto bg-white border px-3 py-2 rounded-lg text-sm">Export CSV</a>
    </div>
   <div className="bg-white border rounded-xl overflow-hidden">
    <div className="overflow-auto max-h-[60vh]">
     <table className="w-full text-sm">
        <thead className="sticky top-0 bg-zinc-50 border-b"><tr><SortHeader column="name" label="Name"/><SortHeader column="company" label="Company" className="hidden md:table-cell"/><SortHeader column="title" label="Title"/><SortHeader column="email" label="Email" className="hidden sm:table-cell"/><SortHeader column="country" label="Country" className="hidden lg:table-cell"/><SortHeader column="linkedin" label="LinkedIn" className="hidden lg:table-cell"/><SortHeader column="score" label="Score"/></tr></thead>
       <tbody>
        {loading ? <tr><td colSpan={7} className="p-8 text-center text-zinc-400">Loading...</td></tr> :
         slice.map((l:any)=><tr key={l.id} onClick={()=>setSel(l)} className="border-t hover:bg-zinc-50 cursor-pointer">
          <td className="p-3"><div className="font-medium">{l.first_name} {l.last_name}</div><div className="text-xs text-zinc-500 md:hidden">{l.company_name}</div></td>
          <td className="p-3 hidden md:table-cell">{l.company_name} <StatusChip v={l.website_status} site/></td>
          <td className="p-3"><span className="bg-blue-50 text-blue-700 px-2 py-1 rounded-full text-xs">{l.position_title}</span></td>
          <td className="p-3 hidden sm:table-cell font-mono text-xs"><div>{l.email}</div><StatusChip v={l.email_status}/></td>
          <td className="p-3 hidden lg:table-cell"><span className="bg-zinc-100 px-2 py-1 rounded-full text-xs">{l.country_code || l.country || '-'}</span></td>
          <td className="p-3 hidden lg:table-cell">{l.linkedin_url ? <a href={l.linkedin_url} target="_blank" rel="noreferrer" onClick={e=>e.stopPropagation()} className="text-blue-600 hover:underline text-xs">LinkedIn ↗</a> : <span className="text-zinc-400 text-xs">-</span>}</td>
          <td className="p-3"><Score v={l.lead_score}/></td>
         </tr>)}
      </tbody>
     </table>
     {!loading && (Array.isArray(slice)?slice:[]).length===0 && <EmptyState title="No leads found" desc="Try adjusting search or start an objective." />}
    </div>
    <div className="flex items-center justify-between p-3 border-t bg-zinc-50 text-xs">
     <span>Showing {total===0?0:(curPage-1)*per+1}–{Math.min(curPage*per,total)} of {total}</span>
     <div className="flex gap-1"><button disabled={curPage<=1} onClick={()=>setPage(p=>p-1)} className="border bg-white px-2 py-1 rounded disabled:opacity-50">Prev</button><span className="px-2 py-1">Page {curPage} of {pages}</span><button disabled={curPage>=pages} onClick={()=>setPage(p=>p+1)} className="border bg-white px-2 py-1 rounded disabled:opacity-50">Next</button></div>
    </div>
   </div>
   <Drawer open={!!sel} onClose={()=>setSel(null)} title="Lead Details">
    {sel && (
     <div className="space-y-4">
       <div><div className="text-lg font-bold">{sel.first_name} {sel.last_name}</div><div className="text-sm text-zinc-500">{sel.position_title} · {sel.company_name}</div></div>
       <Score v={sel.lead_score}/>
        <div className="space-y-2 text-sm"><div><span className="text-zinc-500">Email <StatusChip v={sel.email_status}/></span><div className="font-mono bg-zinc-50 border rounded-lg p-2">{sel.email}</div></div><div><span className="text-zinc-500">Company</span><div className="font-medium">{sel.company_name} {sel.company_domain && `(${sel.company_domain})`} <StatusChip v={sel.website_status} site/></div></div><div><span className="text-zinc-500">Location</span><div>{[sel.city, sel.country_code].filter(Boolean).join(', ') || '-'}</div></div><div><span className="text-zinc-500">Industry</span><div>{sel.industry_name || '-'}</div></div>{sel.linkedin_url && <div><span className="text-zinc-500">LinkedIn</span><div><a href={sel.linkedin_url} target="_blank" rel="noreferrer" className="text-blue-600 hover:underline break-all">{sel.linkedin_url}</a></div></div>}{sel.summary && <div><span className="text-zinc-500">Summary</span><div className="bg-zinc-50 border rounded-lg p-2 text-xs whitespace-pre-wrap">{sel.summary}</div></div>}<div className="bg-blue-50 border border-blue-200 rounded-xl p-3"><div className="text-xs font-bold text-blue-700 mb-1">LinkedIn Intro {sel.outreach_message ? '' : '(generating...)'}</div><div className="text-sm whitespace-pre-wrap">{sel.outreach_message || 'Generating personalized message (2 at a time after email check)...'}</div>{sel.outreach_message && <button onClick={()=>navigator.clipboard.writeText(sel.outreach_message)} className="mt-2 border bg-white px-3 py-1 rounded-lg text-xs">Copy</button>}</div><div><span className="text-zinc-500">Source</span><div>LeadsCaptain</div></div></div>
     </div>
    )}
   </Drawer>
  </div>
 )
}
