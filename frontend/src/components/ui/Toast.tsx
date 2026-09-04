import { useEffect, useState } from 'react'
let pushFn:(m:{title:string,type?:string})=>void = ()=>{}
export function toast(title:string,type='success'){ pushFn({title,type})}
export function ToastContainer(){
 const [items,setItems]=useState<any[]>([])
 useEffect(()=>{ pushFn=(m)=>{ const id=Date.now(); setItems(p=>[...p,{id,...m}]); setTimeout(()=>setItems(p=>p.filter(x=>x.id!==id)),4000)}},[])
 return <div className="fixed bottom-4 right-4 z-50 space-y-2">{(Array.isArray(items)?items:[]).map(i=><div key={i.id} className={`px-4 py-3 rounded-xl shadow-lg text-sm font-medium ${i.type==='error'?'bg-red-600 text-white':'bg-zinc-900 text-white'}`}>{i.title}</div>)}</div>
}
