import { useState } from 'react'
import { Modal } from '../ui/Modal'
import { api } from '../../services/api'
import { toast } from '../ui/Toast'
export function ObjectiveModal({open,onClose,onCreated}:{open:boolean,onClose:()=>void,onCreated:()=>void}){
 const [form,setForm]=useState({name:'',description:'',target_leads:50,minimum_score:0})
 const [loading,setLoading]=useState(false)
 const [err,setErr]=useState<string|null>(null)
  const submit=async(e?:React.MouseEvent)=>{
   e?.preventDefault()
   if(!form.name || !form.description){ setErr('Name and description required'); return}
   setLoading(true); setErr(null)
   try{ await api.createObjective(form); toast('Objective created'); onCreated(); onClose(); setForm({name:'',description:'',target_leads:50,minimum_score:0})}
   catch(e:any){ setErr(e.message)}
   finally{ setLoading(false)}
  }
 return (
  <Modal open={open} onClose={onClose} title="Create Objective">
   <div className="space-y-4">
    <div><label className="text-sm font-medium">Objective name</label><input value={form.name} onChange={e=>setForm({...form,name:e.target.value})} placeholder="CTOs in London SaaS" className="w-full mt-1 border rounded-lg px-3 py-2 text-sm"/></div>
    <div><label className="text-sm font-medium">Description</label><textarea value={form.description} onChange={e=>setForm({...form,description:e.target.value})} placeholder="Find 500 CTOs..." rows={3} className="w-full mt-1 border rounded-lg px-3 py-2 text-sm"/></div>
    <div className="grid grid-cols-2 gap-3">
     <div><label className="text-sm font-medium">Target leads</label><input type="number" min={1} value={form.target_leads} onChange={e=>setForm({...form,target_leads:parseInt(e.target.value)||0})} className="w-full mt-1 border rounded-lg px-3 py-2 text-sm"/></div>
     <div><label className="text-sm font-medium">Minimum score <span className="text-zinc-400 font-normal">(0 = off)</span></label><input type="number" min={0} max={100} value={form.minimum_score} onChange={e=>setForm({...form,minimum_score:Math.max(0,Math.min(100,parseInt(e.target.value)||0))})} className="w-full mt-1 border rounded-lg px-3 py-2 text-sm"/></div>
    </div>
    {err && <div className="bg-red-50 border border-red-200 text-red-700 p-3 rounded-lg text-sm">{err}</div>}
     <div className="flex justify-end gap-2"><button type="button" onClick={onClose} className="border px-4 py-2 rounded-lg text-sm">Cancel</button><button type="button" onClick={submit} disabled={loading} className="bg-zinc-900 text-white px-4 py-2 rounded-lg text-sm disabled:opacity-50">{loading?'Creating...':'Create objective'}</button></div>
   </div>
  </Modal>
 )
}
