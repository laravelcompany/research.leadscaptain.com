import { useMemo, useState } from 'react'
export function EventStream({events,state,clear,compact=false}:{events:any[],state:string,clear:()=>void,compact?:boolean}){
 const [filter,setFilter]=useState(''); const [search,setSearch]=useState(''); const [copied,setCopied]=useState<number|null>(null)
 const color:any={connected:'bg-emerald-500',connecting:'bg-amber-500',reconnecting:'bg-amber-500',disconnected:'bg-red-500'}
 const types=useMemo(()=>[...new Set(events.map(e=>e?.type).filter(Boolean))].sort() as string[],[events])
 const shown=events.filter(e=>(!filter||e.type===filter)&&(!search||JSON.stringify(e).toLowerCase().includes(search.toLowerCase())))
 const copy=async(e:any,i:number)=>{await navigator.clipboard.writeText(JSON.stringify(e,null,2));setCopied(i);setTimeout(()=>setCopied(null),1200)}
 return <div className="border rounded-2xl overflow-hidden bg-zinc-950 text-zinc-200">
  <div className="flex items-center justify-between px-4 py-3 border-b border-zinc-800 gap-3 flex-wrap">
   <div className="flex items-center gap-2 text-xs font-mono"><span className={`w-2 h-2 rounded-full ${color[state]||'bg-zinc-500'}`}/><span>{state.toUpperCase()}</span><span className="text-zinc-500">•</span><span>{shown.length}{shown.length!==events.length?`/${events.length}`:''} events</span></div>
   <div className="flex items-center gap-2 flex-wrap">
    <input aria-label="Search events" value={search} onChange={e=>setSearch(e.target.value)} placeholder="Search payloads..." className="text-xs bg-zinc-900 border border-zinc-700 rounded px-2 py-1.5"/>
    <select aria-label="Filter event type" value={filter} onChange={e=>setFilter(e.target.value)} className="text-xs bg-zinc-900 border border-zinc-700 rounded px-2 py-1.5"><option value="">All types</option>{types.map(t=><option key={t}>{t}</option>)}</select>
    <button onClick={clear} className="text-xs border border-zinc-700 px-2 py-1.5 rounded hover:bg-zinc-800">Clear local log</button>
   </div>
  </div>
  <div className={`${compact?'max-h-80':'max-h-[60vh]'} overflow-auto p-2 space-y-1`}>
   {shown.map((e,i)=><details key={`${e.timestamp||''}-${i}`} className="bg-zinc-900 border border-zinc-800 rounded-xl p-2 group"><summary className="text-xs font-mono text-emerald-400 cursor-pointer flex gap-2"><span>{e.type||'event'}</span><time className="text-zinc-500 ml-auto">{e.timestamp?new Date(e.timestamp).toLocaleTimeString():''}</time></summary><div className="flex justify-end mt-2"><button onClick={()=>copy(e,i)} className="text-[11px] border border-zinc-700 px-2 py-1 rounded">{copied===i?'Copied':'Copy payload'}</button></div><pre className="text-xs text-zinc-300 whitespace-pre-wrap break-all mt-1">{JSON.stringify(e.payload??e,null,2)}</pre></details>)}
   {!shown.length&&<div className="text-center py-8 text-zinc-500 text-sm">{events.length?'No events match the filters':'Waiting for objective activity...'}</div>}
  </div>
 </div>
}
