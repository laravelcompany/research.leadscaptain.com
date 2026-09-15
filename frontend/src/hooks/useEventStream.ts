import { useEffect, useRef, useState } from 'react'
export type ConnState='connecting'|'connected'|'disconnected'|'reconnecting'
export function useEventStream(onEvent:(e:any)=>void, objectiveId?:string){
 const [events,setEvents]=useState<any[]>([])
 const [state,setState]=useState<ConnState>('connecting')
 const callback=useRef(onEvent); callback.current=onEvent
 const retryRef=useRef(0)
 useEffect(()=>{
  let closed=false; let timer:number|undefined; let es:EventSource|undefined
  setEvents([]); retryRef.current=0
  const connect=()=>{
   if(closed) return
   setState(retryRef.current===0?'connecting':'reconnecting')
   const q=objectiveId?`?objective_id=${encodeURIComponent(objectiveId)}`:''
   es=new EventSource('/api/v1/events'+q)
   es.onopen=()=>{setState('connected');retryRef.current=0}
   es.onmessage=e=>{
    try{const d=JSON.parse(e.data);setEvents(p=>[d,...p].slice(0,500));callback.current(d)}catch{}
   }
   es.onerror=()=>{es.close();if(closed)return;setState('disconnected');retryRef.current++;timer=window.setTimeout(connect,Math.min(30000,1000*Math.pow(1.5,retryRef.current)))}
  }
  connect()
  return()=>{closed=true;if(timer)clearTimeout(timer);es?.close()}
 },[objectiveId])
 return {events,state,clear:()=>setEvents([])}
}
