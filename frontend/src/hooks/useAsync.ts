import { useCallback, useEffect, useState } from 'react'
export function useAsync<T>(fn:()=>Promise<T>, deps:any[]=[]){
 const [data,setData]=useState<T|null>(null)
 const [loading,setLoading]=useState(true)
 const [error,setError]=useState<string|null>(null)
 const refresh=useCallback(async()=>{
  setLoading(true); setError(null)
  try{ const v=await fn(); setData(v)}catch(e:any){ setError(e.message||String(e))}
  finally{ setLoading(false)}
 },deps)
 useEffect(()=>{ refresh()},[refresh])
 return {data,loading,error,refresh}
}
