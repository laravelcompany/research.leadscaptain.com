export function Header({onRefresh,lastUpdated,conn}:{onRefresh:()=>void,lastUpdated:string,conn?:string}){
 const dot:any={connected:'bg-emerald-500',connecting:'bg-amber-500',reconnecting:'bg-amber-500 animate-pulse',disconnected:'bg-red-500'}
 return (
  <header className="sticky top-0 z-10 bg-white/80 backdrop-blur border-b">
   <div className="px-4 md:px-6 py-3 flex items-center gap-3">
    <div className="flex-1 min-w-0"><h1 className="font-bold text-sm md:text-base">Autonomous Lead Research</h1><p className="text-xs text-zinc-500 hidden sm:block">AI agent actively researching, scoring & qualifying leads</p></div>
    {conn && <span title={`Live event stream: ${conn}`} className="hidden sm:inline-flex items-center gap-1.5 text-xs text-zinc-500"><span className={`w-2 h-2 rounded-full ${dot[conn]||'bg-zinc-400'}`}/>{conn==='connected'?'Live':conn}</span>}
    <div className="hidden sm:block text-xs text-zinc-400">{lastUpdated}</div>
    <button onClick={onRefresh} className="border bg-white px-3 py-1.5 rounded-lg text-xs font-medium hover:bg-zinc-50">↻ Refresh</button>
    <a href="/api/v1/leads/export" className="bg-emerald-600 text-white px-3 py-1.5 rounded-lg text-xs font-medium hover:bg-emerald-700">Export CSV</a>
   </div>
  </header>
 )
}
