export function RunList({iterations}:{iterations:any[]}){
 const list=Array.isArray(iterations)?iterations:[]
 const runs=new Map<number,any[]>()
 list.forEach((it:any)=>{ if(!runs.has(it.run_id)) runs.set(it.run_id,[]); runs.get(it.run_id)!.push(it)})
 const entries=[...runs.entries()].slice(0,20)
 if(entries.length===0) return <div className="text-center py-12 text-zinc-400">No runs yet</div>
 return (
  <div className="grid md:grid-cols-2 gap-3 max-h-[65vh] overflow-auto">
   {(Array.isArray(entries)?entries:[]).map(([id,its])=>{
    const last=its[0]; const count=its.length
    return (
     <div key={id} className="bg-white border rounded-2xl p-4">
      <div className="font-mono text-sm font-bold">RUN-{String(id).padStart(5,'0')}</div>
      <div className="text-xs text-zinc-500">Iterations {count} • Last: {last.action} • {last.status}</div>
      <div className="mt-3 space-y-1 max-h-32 overflow-auto">
       {its.slice(0,8).map((it:any)=><div key={it.id} className="flex gap-2 text-xs"><span className="text-zinc-400">#{it.iteration}</span><span className="font-medium">{it.action}</span><span className={it.status==='completed'?'text-emerald-600':'text-amber-600'}>{it.status}</span></div>)}
      </div>
     </div>
    )
   })}
  </div>
 )
}
