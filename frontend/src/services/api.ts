async function apiFetch<T>(path:string, opts:RequestInit={}):Promise<T>{
 const hasBody=!!opts.body
 const headers:any={...(opts.headers as any||{})}
 if(hasBody) headers['Content-Type']='application/json'
 const res=await fetch(path, {...opts, headers})
 if(!res.ok){
  const text=await res.text().catch(()=>'')
  let msg=text
  try{ const j=JSON.parse(text); msg=j.error?.message || j.error || text }catch{}
  throw new Error(msg || `HTTP ${res.status}`)
 }
 const ct=res.headers.get('content-type')||''
 if(ct.includes('text/csv') || ct.includes('text/plain')) return (await res.text() as unknown as T)
 return res.json() as Promise<T>
}
export const api={
 stats:()=>apiFetch<any>('/api/v1/stats'),
  leads:(params:Record<string,string>={})=>{
   const clean:Record<string,string>={}; for(const [k,v] of Object.entries(params)) if(v) clean[k]=v
   const q=new URLSearchParams(clean).toString()
   return apiFetch<{data:any[],page:number,per_page:number}>('/api/v1/leads'+(q?'?'+q:''))
 },
 objectives:()=>apiFetch<any[]>('/api/v1/objectives'),
 createObjective:(body:any)=>apiFetch<any>('/api/v1/objectives',{method:'POST',body:JSON.stringify(body)}),
  objective:(id:number)=>apiFetch<any>(`/api/v1/objectives/${id}`),
  deleteObjective:(id:number)=>apiFetch<any>(`/api/v1/objectives/${id}`,{method:'DELETE'}),
 startObjective:(id:number)=>apiFetch<any>(`/api/v1/objectives/${id}/start`,{method:'POST'}),
 pauseObjective:(id:number)=>apiFetch<any>(`/api/v1/objectives/${id}/pause`,{method:'POST'}),
 stopObjective:(id:number)=>apiFetch<any>(`/api/v1/objectives/${id}/stop`,{method:'POST'}),
 resumeObjective:(id:number)=>apiFetch<any>(`/api/v1/objectives/${id}/resume`,{method:'POST'}),
  clearLeads:()=>apiFetch<any>('/api/v1/leads',{method:'DELETE'}),
  iterations:()=>apiFetch<any[]>('/api/v1/iterations'),
 searchHistory:()=>apiFetch<any[]>('/api/v1/search-history'),
 exportUrl:(params:Record<string,string>={})=>{
  const q=new URLSearchParams(params).toString()
  return '/api/v1/leads/export'+(q?'?'+q:'')
 }
}
