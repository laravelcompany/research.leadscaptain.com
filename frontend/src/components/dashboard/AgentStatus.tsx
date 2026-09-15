export function AgentStatus({objectives, stats}:{objectives:any[],stats:any}){
 const list=Array.isArray(objectives)?objectives:[]
 const running=list.find((o:any)=>o.status==='running')
 const status=running?'RUNNING':'IDLE'
 const target=running?.target_leads ?? 0
 const done=running?.qualified ?? running?.leads_discovered ?? 0
 const pct=target ? Math.min(100, Math.round(done/target*100)) : 0
 return (
  <div className="bg-zinc-900 text-white rounded-2xl p-5">
   <div className="flex items-center gap-2 text-xs tracking-widest font-semibold"><span className={`w-2 h-2 rounded-full ${status==='RUNNING'?'bg-emerald-400 animate-pulse':'bg-zinc-500'}`}></span> AGENT {status}</div>
   {running ? (
    <div className="mt-3">
     <div className="font-semibold">{running.name}</div><div className="text-sm text-zinc-400">{running.description}</div>
     <div className="grid grid-cols-3 gap-3 mt-4 text-center">
      <div className="bg-white/10 rounded-xl p-3"><div className="text-lg font-bold">{stats.leads ?? 0}</div><div className="text-xs text-zinc-400">Discovered</div></div>
      <div className="bg-white/10 rounded-xl p-3"><div className="text-lg font-bold">{stats.qualified ?? 0}</div><div className="text-xs text-zinc-400">Qualified</div></div>
      <div className="bg-white/10 rounded-xl p-3"><div className="text-lg font-bold">{running.target_leads ?? '-'}</div><div className="text-xs text-zinc-400">Target</div></div>
     </div>
     {target ? <div className="mt-4"><div className="flex justify-between text-xs text-zinc-400"><span>Progress to target</span><span>{done}/{target} · {pct}%</span></div><div className="h-2 bg-white/10 rounded-full overflow-hidden mt-1"><div className="h-full bg-emerald-400 transition-all duration-500" style={{width:pct+'%'}}/></div></div> : null}
    </div>
   ) : (
    <div className="mt-3 text-sm text-zinc-400">No active research. Start an objective to begin autonomous discovery.</div>
   )}
  </div>
 )
}
