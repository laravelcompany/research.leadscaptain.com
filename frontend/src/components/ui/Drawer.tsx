import { useEffect } from 'react'
export function Drawer({open,onClose,title,children}:{open:boolean,onClose:()=>void,title:string,children:any}){
 useEffect(()=>{
  const h=(e:KeyboardEvent)=>{ if(e.key==='Escape') onClose()}
  if(open) document.addEventListener('keydown',h)
  return()=>document.removeEventListener('keydown',h)
 },[open,onClose])
 if(!open) return null
 return (
  <div className="fixed inset-0 z-50 flex justify-end">
   <div className="absolute inset-0 bg-black/30" onClick={onClose} />
   <div className="relative bg-white w-full max-w-md h-full overflow-auto shadow-2xl">
    <div className="sticky top-0 bg-white border-b px-6 py-4 flex justify-between items-center"><h3 className="font-semibold">{title}</h3><button onClick={onClose} className="w-8 h-8 grid place-items-center rounded-full hover:bg-zinc-100">✕</button></div>
    <div className="p-6">{children}</div>
   </div>
  </div>
 )
}
