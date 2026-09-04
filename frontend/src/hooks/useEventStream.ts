import { useEffect, useRef, useState } from 'react'
export type ConnState='connecting'|'connected'|'disconnected'|'reconnecting'
export function useEventStream(onEvent:(e:any)=>void){
 const [events,setEvents]=useState<any[]>([])
 const [state,setState]=useState<ConnState>('connecting')
 const esRef=useRef<EventSource|null>(null)
 const retryRef=useRef(0)
 useEffect(()=>{
  let closed=false
  let timer:any
  const connect=()=>{
   if(closed) return
   setState(retryRef.current===0?'connecting':'reconnecting')
   const es=new EventSource('/api/v1/events')
   esRef.current=es
   es.onopen=()=>{ setState('connected'); retryRef.current=0}
   es.onmessage=e=>{
    try{ const d=JSON.parse(e.data); setEvents(p=>[d,...p].slice(0,200)); onEvent(d)}catch{ setEvents(p=>[e.data,...p].slice(0,200))}
   }
   es.onerror=()=>{
    es.close(); setState('disconnected')
    if(closed) return
    retryRef.current++
    const delay=Math.min(30000, 1000*Math.pow(1.5, retryRef.current))
    timer=setTimeout(connect, delay)
   }
  }
  connect()
  return()=>{ closed=true; clearTimeout(timer); esRef.current?.close()}
 },[])
 const clear=()=>setEvents([])
 const pause=()=>esRef.current?.close()
 return {events,state,clear,pause}
}
