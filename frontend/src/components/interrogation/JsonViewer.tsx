import { useState } from 'react'
export function JsonViewer({data}:{data:string}){
 const [copied,setCopied]=useState(false)
 let pretty=data
 try{ pretty=JSON.stringify(JSON.parse(data),null,2)}catch{}
 const copy=()=>{ navigator.clipboard.writeText(pretty); setCopied(true); setTimeout(()=>setCopied(false),1500)}
 const [open,setOpen]=useState(false)
 return (
  <div className="border rounded-xl overflow-hidden">
   <div className="flex justify-between items-center bg-zinc-50 px-3 py-2 border-b"><span className="text-xs font-semibold">JSON</span><button onClick={copy} className="text-xs border bg-white px-2 py-1 rounded">{copied?'Copied':'Copy JSON'}</button></div>
   <button onClick={()=>setOpen(!open)} className="w-full text-left px-3 py-2 text-xs bg-white hover:bg-zinc-50">{open?'▼ Hide':'▶ Show'} payload ({pretty.length} chars)</button>
   {open && <pre className="bg-zinc-900 text-emerald-300 p-3 text-xs overflow-auto max-h-80 whitespace-pre-wrap">{pretty || '-'}</pre>}
  </div>
 )
}
