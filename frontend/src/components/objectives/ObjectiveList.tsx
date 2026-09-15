import { useState } from 'react'
import { Badge } from '../ui/Badge'
import { Modal } from '../ui/Modal'
import { api } from '../../services/api'
import { toast } from '../ui/Toast'
export function ObjectiveList({objectives,refresh}:{objectives:any[],refresh:()=>void}){
 const [busy,setBusy]=useState<number|null>(null)
 const [confirm,setConfirm]=useState<{id:number,action:string}|null>(null)
 const run=async(id:number,action:string)=>{
  setBusy(id)
  try{
   if(action==='start') await api.startObjective(id);
   else if(action==='resume') await api.resumeObjective(id);
   else if(action==='pause') await api.pauseObjective(id);
   else if(action==='stop') await api.stopObjective(id);
   else if(action==='delete') await api.deleteObjective(id);
   toast(`Objective ${action}d`); refresh()
  }catch(e:any){ toast(e.message,'error')}
  finally{ setBusy(null); setConfirm(null)}
 }
 return (
  <div className="space-y-3">
   {(Array.isArray(objectives)?objectives:[]).map(o=>{
     const pct=o.progress ?? (o.target_leads ? Math.min(100, Math.round(((o.qualified ?? o.leads_discovered ?? 0))/o.target_leads*100)) : 0)
     const paused=o.status==='paused'
    return (
     <div key={o.id} className="bg-white border rounded-2xl p-4">
      <div className="flex justify-between items-start gap-3">
       <div className="min-w-0 flex-1"><div className="font-semibold flex items-center gap-2 flex-wrap">{o.name} <Badge variant={o.status==='running'?'success':o.status==='completed'?'info':o.status==='failed'?'danger':paused?'warning':'default'}>{o.status}</Badge>{o.minimum_score ? <span className="text-xs text-zinc-400 font-normal">min score {o.minimum_score}</span> : null}</div><div className="text-sm text-zinc-500 truncate">{o.description}</div></div>
        <div className="flex gap-1.5 shrink-0 flex-wrap justify-end">
         {paused ? (
          <button disabled={busy===o.id} onClick={()=>run(o.id,'resume')} className="bg-emerald-600 text-white px-3 py-1.5 rounded-lg text-xs disabled:opacity-50 hover:bg-emerald-700">{busy===o.id?'...':'Resume'}</button>
         ) : (
          <button disabled={busy===o.id || o.status==='running'} onClick={()=>run(o.id,'start')} className="bg-zinc-900 text-white px-3 py-1.5 rounded-lg text-xs disabled:opacity-50">{busy===o.id?'...':'Start'}</button>
         )}
         <button disabled={busy===o.id} onClick={()=>setConfirm({id:o.id,action:'pause'})} className="border bg-white px-3 py-1.5 rounded-lg text-xs">Pause</button>
         <button disabled={busy===o.id} onClick={()=>setConfirm({id:o.id,action:'stop'})} className="border bg-white px-3 py-1.5 rounded-lg text-xs">Stop</button>
         <button disabled={busy===o.id} onClick={()=>setConfirm({id:o.id,action:'delete'})} className="border bg-white hover:bg-red-50 text-red-600 px-3 py-1.5 rounded-lg text-xs">Delete</button>
        </div>
      </div>
       {o.target_leads && <div className="mt-3"><div className="flex justify-between text-xs font-medium text-zinc-600"><span>Progress {o.qualified ?? o.leads_discovered ?? 0}/{o.target_leads} {o.iteration_count ? `· ${o.iteration_count} iters` : ''} · {o.status}</span><span>{pct}%</span></div><div className="h-2 bg-zinc-100 rounded-full overflow-hidden mt-1"><div className={`h-full transition-all duration-500 ${o.status==='running'?'bg-emerald-600 animate-pulse':o.status==='completed'?'bg-blue-600':'bg-zinc-900'}`} style={{width:pct+'%'}}/></div></div>}
     </div>
    )
   })}
   {(Array.isArray(objectives)?objectives:[]).length===0 && <div className="text-center py-12 text-zinc-400">No objectives yet. Create your first research objective.</div>}
   <Modal open={!!confirm} onClose={()=>setConfirm(null)} title={`${confirm?.action} objective?`}>
    <p className="text-sm text-zinc-600">This will {confirm?.action} the autonomous research run.</p>
    <div className="flex justify-end gap-2 mt-6"><button onClick={()=>setConfirm(null)} className="border px-4 py-2 rounded-lg text-sm">Cancel</button><button onClick={()=>confirm && run(confirm.id, confirm.action)} className="bg-red-600 text-white px-4 py-2 rounded-lg text-sm">Confirm</button></div>
   </Modal>
  </div>
 )
}
