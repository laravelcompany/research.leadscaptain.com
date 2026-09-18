const items=[
 {id:'dashboard', label:'Dashboard', icon:'▦'},
 {id:'leads', label:'Leads', icon:'◈'},
 {id:'companies', label:'Companies', icon:'▣'},
 {id:'websites', label:'Websites', icon:'◉'},
 {id:'objectives', label:'Objectives', icon:'◎'},
 {id:'runs', label:'Runs', icon:'⟡'},
 {id:'interrogation', label:'Interrogation', icon:'⬢'},
 {id:'live', label:'Live Events', icon:'●'},
]
export function Sidebar({active,onChange,collapsed}:{active:string,onChange:(id:string)=>void,collapsed?:boolean}){
 return (
  <aside className={`${collapsed?'w-16':'w-64'} hidden md:flex flex-col border-r bg-white shrink-0 transition-all`}>
   <div className="px-4 py-5 border-b"><div className="font-bold text-sm tracking-tight">RESEARCH AGENT</div><div className="text-xs text-zinc-500">Autonomous Ops</div></div>
   <nav className="flex-1 p-2 space-y-1 overflow-auto">
    {(Array.isArray(items)?items:[]).map(it=><button key={it.id} onClick={()=>onChange(it.id)} className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-xl text-sm font-medium transition ${active===it.id?'bg-zinc-900 text-white':'hover:bg-zinc-100 text-zinc-700'}`}><span className="w-5 text-center">{it.icon}</span>{!collapsed && it.label}</button>)}
   </nav>
   <div className="p-3 border-t text-xs text-zinc-400">v1.0 • Connected</div>
  </aside>
 )
}
export function MobileNav({active,onChange}:{active:string,onChange:(id:string)=>void}){
 return (
  <div className="md:hidden flex gap-1 overflow-auto p-2 bg-white border-b sticky top-0 z-20">
   {(Array.isArray(items)?items:[]).map(it=><button key={it.id} onClick={()=>onChange(it.id)} className={`shrink-0 px-3 py-1.5 rounded-full text-xs font-medium ${active===it.id?'bg-zinc-900 text-white':'bg-zinc-100'}`}>{it.label}</button>)}
  </div>
 )
}
