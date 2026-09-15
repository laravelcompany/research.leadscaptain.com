import { useMemo, useState } from 'react'
export function EventStream({events,state,clear}:{events:any[],state:string,clear:()=>void}){
 const color:any={connected:'bg-emerald-500', connecting:'bg-amber-500', reconnecting:'bg-amber-500', disconnected:'bg-red-500'}
 const [filter,setFilter]=useState('')
 const types=useMemo(()=>[...new Set((Array.isArray(events)?events:[]).map((e:any)=>e?.type).filter(Boolean))].sort() as string[],[events])
 const shown=filter ? (Array.isArray(events)?events:[]).filter((e:any)=>e?.type===filter) : (Array.isArray(events)?events:[])
 return (
  <div className="border rounded-2xl overflow-hidden bg-zinc-950">
   <div className="flex items-center justify-between px-4 py-3 border-b border-zinc-800 gap-3 flex-wrap">
    <div className="flex items-center gap-2 text-xs font-mono text-zinc-300"><span className={`w-2 h-2 rounded-full ${color[state]||'bg-zinc-500'}`}/> {state.toUpperCase()} • {shown.length}{filter?`/${events.length}`:''} events</div>
    <div className="flex items-center gap-2">
     <select value={filter} onChange={e=>setFilter(e.target.value)} className="text-xs bg-zinc-900 border border-zinc-700 text-zinc-300 rounded px-2 py-1">
      <option value="">All types</option>
      {types.map(t=><option key={t} value={t}>{t}</option>)}
     </select>
     <button onClick={clear} className="text-xs border border-zinc-700 text-zinc-300 px-2 py-1 rounded hover:bg-zinc-800">Clear</button>
    </div>
   </div>
   <div className="max-h-[60vh] overflow-auto p-2 space-y-1">
    {shown.map((e:any,i:number)=><details key={i} className="bg-zinc-900 border border-zinc-800 rounded-xl p-2"><summary className="text-xs font-mono text-emerald-400 cursor-pointer">{e.type || 'event'} <span className="text-zinc-500">{e.timestamp || ''}</span></summary><pre className="text-xs text-zinc-300 whitespace-pre-wrap mt-2">{JSON.stringify(e,null,2).slice(0,4000)}</pre></details>)}
    {shown.length===0 && <div className="text-center py-8 text-zinc-500 text-sm">{filter?`No ${filter} events`:'No events yet'}</div>}
   </div>
  </div>
 )
}
