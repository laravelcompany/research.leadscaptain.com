import { useState } from 'react'
import { Badge } from '../ui/Badge'
import { JsonViewer } from './JsonViewer'
export function IterationCard({it}:{it:any}){
 const [open,setOpen]=useState(false)
 return (
  <div className="border rounded-2xl overflow-hidden bg-white">
   <button onClick={()=>setOpen(!open)} className="w-full flex justify-between items-center px-4 py-3 hover:bg-zinc-50 text-left">
    <span className="font-mono text-sm">#{it.iteration} {it.action || 'pending'}</span>
    <span className="flex items-center gap-2"><Badge variant={it.status==='completed'?'success':it.status==='failed'?'danger':'warning'}>{it.status}</Badge><span className="text-xs text-zinc-400">{open?'▼':'▶'}</span></span>
   </button>
   {open && (
    <div className="p-4 space-y-4 border-t bg-zinc-50/50">
     {it.reflection && <div><div className="text-xs font-bold text-zinc-600 mb-1">AI Reasoning</div><div className="bg-amber-50 border border-amber-200 rounded-xl p-3 text-sm">{it.reflection}</div></div>}
     <div><div className="text-xs font-bold text-zinc-600 mb-1">1 Prompt → AI</div><pre className="bg-blue-50 border border-blue-200 p-3 rounded-xl text-xs whitespace-pre-wrap">{it.prompt || '-'}</pre></div>
     <div><div className="text-xs font-bold text-zinc-600 mb-1">2 AI Raw Response</div><JsonViewer data={it.ai_raw || ''} /></div>
     <div><div className="text-xs font-bold text-zinc-600 mb-1">3 Tool Call</div><div className="bg-white border rounded-xl p-3 font-mono text-xs">tool: {it.action} params: {it.tool_params || '{}'}</div></div>
     <div><div className="text-xs font-bold text-zinc-600 mb-1">4 API Response</div><JsonViewer data={it.action_result || ''} /></div>
    </div>
   )}
  </div>
 )
}
