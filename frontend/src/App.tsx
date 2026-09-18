import { useEffect, useState } from 'react'
import { api, setOnUnauthorized } from './services/api'
import { LoginScreen } from './components/auth/LoginScreen'
import { useEventStream } from './hooks/useEventStream'
import { Sidebar, MobileNav } from './components/layout/Sidebar'
import { Header } from './components/layout/Header'
import { StatCard } from './components/dashboard/StatCard'
import { AgentStatus } from './components/dashboard/AgentStatus'
import { LeadTable } from './components/leads/LeadTable'
import { ObjectiveList } from './components/objectives/ObjectiveList'
import { ObjectiveModal } from './components/objectives/ObjectiveModal'
import { RunList } from './components/runs/RunList'
import { IterationCard } from './components/interrogation/IterationCard'
import { EventStream } from './components/events/EventStream'
import { ToastContainer, toast } from './components/ui/Toast'
import { ErrorBoundary } from './components/ui/ErrorBoundary'
import { ObjectiveLeadsPage } from './components/objectives/ObjectiveLeadsPage'
import { CompaniesPage } from './components/companies/CompaniesPage'
import { WebsitesPage } from './components/websites/WebsitesPage'
import { ToolsPage } from './components/tools/ToolsPage'
import { BrowserRouter, useLocation, useNavigate } from 'react-router-dom'

function Application(){
 const location=useLocation(); const navigate=useNavigate()
 const [authState,setAuthState]=useState<'loading'|'login'|'ready'>('loading')
 const [authUser,setAuthUser]=useState('')
 const initialPath=location.pathname
 const [tab,setTabState]=useState(initialPath==='/objectives'?'objectives':initialPath==='/leads'?'leads':initialPath==='/runs'?'runs':initialPath==='/interrogation'?'interrogation':initialPath==='/events'?'live':initialPath==='/companies'?'companies':initialPath==='/websites'?'websites':initialPath==='/tools'?'tools':'dashboard')
 const setTab=(next:string)=>{setTabState(next);navigate(next==='dashboard'?'/':next==='live'?'/events':'/'+next)}
 const [stats,setStats]=useState<any>(null)
 const [statsLoading,setStatsLoading]=useState(true)
 const [objectives,setObjectives]=useState<any[]>([])
 const [objLoading,setObjLoading]=useState(true)
 const [leads,setLeads]=useState<any[]>([])
 const [leadsLoading,setLeadsLoading]=useState(false)
 const [search,setSearch]=useState('')
 const [iters,setIters]=useState<any[]>([])
 const [itersLoading,setItersLoading]=useState(true)
 const [modalOpen,setModalOpen]=useState(false)
 const [lastUpdated,setLastUpdated]=useState('')

 const refreshStats=async()=>{ setStatsLoading(true); try{ setStats(await api.stats())}catch(e:any){ toast(e.message,'error')}finally{ setStatsLoading(false)}}
 const refreshObjectives=async()=>{ setObjLoading(true); try{ setObjectives(await api.objectives())}catch(e:any){ toast(e.message,'error')}finally{ setObjLoading(false)}}
 const refreshLeads=async()=>{ setLeadsLoading(true); try{ const r:any=await api.leads({per_page:'100',search:search.trim()}); console.log('leads',r); setLeads(r.data||r||[])}catch(e:any){ console.error(e); toast(e.message,'error')}finally{ setLeadsLoading(false)}}
 const refreshIters=async()=>{ setItersLoading(true); try{ const r:any=await api.iterations(); setIters(Array.isArray(r)?r:r?.data||[])}catch(e:any){ toast(e.message,'error')}finally{ setItersLoading(false)}}
 const refreshAll=()=>{ refreshStats(); refreshObjectives(); refreshLeads(); refreshIters(); setLastUpdated('Last updated '+new Date().toLocaleTimeString())}

 useEffect(()=>{
  setOnUnauthorized(()=>setAuthState('login'))
  api.auth.me()
   .then(r=>{ if(r.auth_required && !r.authenticated){ setAuthState('login') } else { setAuthUser(r.username||''); setAuthState('ready') } })
   .catch(()=>setAuthState('login'))
 },[])

 useEffect(()=>{ if(authState==='ready') refreshAll()},[authState])

 const logout=async()=>{ try{ await api.auth.logout() }catch{} setAuthUser(''); setAuthState('login') }


 const {events,state,clear}=useEventStream((e)=>{
  if(e.type==='lead.created' || e.type==='api.response') refreshLeads(), refreshStats()
  else if(e.type==='agent.started' || e.type==='agent.completed') refreshObjectives(), refreshStats()
  else if(e.type?.startsWith('iteration')) refreshIters()
 })

  if(authState==='loading') return <div className="min-h-screen bg-zinc-50 flex items-center justify-center text-sm text-zinc-400">Loading...</div>
  if(authState==='login') return <><LoginScreen onLogin={(u)=>{ setAuthUser(u); setAuthState('ready') }} /><ToastContainer /></>

 const objectiveMatch=location.pathname.match(/^\/objectives\/([^/]+)\/leads\/?$/)
 return (
   <ErrorBoundary><div className="min-h-screen bg-zinc-50 flex">
   <Sidebar active={objectiveMatch?'objectives':tab} onChange={setTab} />
   <div className="flex-1 min-w-0 flex flex-col">
    <Header onRefresh={refreshAll} lastUpdated={lastUpdated} conn={state} authUser={authUser} onLogout={logout} />
    <MobileNav active={objectiveMatch?'objectives':tab} onChange={setTab} />
    <main className="flex-1 p-4 md:p-6 space-y-6 overflow-auto">
     {objectiveMatch ? <ObjectiveLeadsPage id={objectiveMatch[1]} onBack={()=>{setTabState('objectives');navigate('/objectives')}} /> : <>
     {tab==='dashboard' && (
      <>
       <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <StatCard label="TOTAL LEADS" value={stats?.leads} loading={statsLoading} sub={`${stats?.qualified ?? 0} qualified`} />
        <StatCard label="QUALIFIED" value={stats?.qualified} loading={statsLoading} sub={`${stats?.leads ? Math.round(stats.qualified/stats.leads*100):0}% rate`} />
        <StatCard label="RUNS" value={stats?.runs} loading={statsLoading} />
        <StatCard label="OBJECTIVES" value={(Array.isArray(objectives)?objectives:[]).length} loading={objLoading} />
       </div>
       <AgentStatus objectives={objectives} stats={stats||{}} />
       <div className="bg-white border rounded-2xl p-4">
        <div className="flex items-center justify-between mb-3"><h3 className="font-semibold">Recent Leads</h3><button onClick={()=>setTab('leads')} className="text-xs text-zinc-500 hover:text-zinc-900">View all →</button></div>
        <LeadTable leads={(Array.isArray(leads)?leads:[]).slice(0,5)} loading={leadsLoading} search={search} setSearch={setSearch} onSearch={refreshLeads} />
       </div>
      </>
     )}
     {tab==='leads' && <LeadTable leads={leads} loading={leadsLoading} search={search} setSearch={setSearch} onSearch={refreshLeads} />}
     {tab==='companies' && <CompaniesPage />}
     {tab==='websites' && <WebsitesPage />}
     {tab==='tools' && <ToolsPage />}
     {tab==='objectives' && (
      <div className="space-y-3">
       <div className="flex justify-between items-center"><h2 className="font-bold text-lg">Objectives</h2><button onClick={()=>setModalOpen(true)} className="bg-zinc-900 text-white px-4 py-2 rounded-xl text-sm">+ New Objective</button></div>
       <ObjectiveList objectives={objectives} refresh={refreshObjectives} onOpen={id=>navigate(`/objectives/${id}/leads`)} />
      </div>
     )}
     {tab==='runs' && <RunList iterations={iters} />}
     {tab==='interrogation' && (
      <div className="space-y-3">
       <h2 className="font-bold text-lg">AI Interrogation</h2>
       <p className="text-sm text-zinc-500">Each iteration shows prompt then AI then tool then API response. Collapsed by default.</p>
       {itersLoading ? <div className="text-center py-8 text-zinc-400">Loading...</div> : <div className="space-y-3 max-h-[70vh] overflow-auto pr-1">{(Array.isArray(iters)?iters:[]).map((it:any)=><IterationCard key={it.id} it={it} />)}{(Array.isArray(iters)?iters:[]).length===0 && <div className="text-center py-8 text-zinc-400">No iterations yet</div>}</div>}
      </div>
     )}
     {tab==='live' && <EventStream events={events} state={state} clear={clear} />}
     </>}
    </main>
   </div>
   <ObjectiveModal open={modalOpen} onClose={()=>setModalOpen(false)} onCreated={()=>{ refreshObjectives(); setTab('objectives'); toast('Objective created — showing Objectives'); }} />
   <ToastContainer />
   </div></ErrorBoundary>
  )
}

export default function App(){ return <BrowserRouter><Application/></BrowserRouter> }
