// Set by App: called whenever any API request comes back 401, so the UI
// can bounce back to the login screen mid-session.
export let onUnauthorized:()=>void=()=>{}
export function setOnUnauthorized(cb:()=>void){ onUnauthorized=cb }

async function apiFetch<T>(path:string, opts:RequestInit={}):Promise<T>{
 const hasBody=!!opts.body
 const headers:any={...(opts.headers as any||{})}
 if(hasBody) headers['Content-Type']='application/json'
 const res=await fetch(path, {...opts, headers})
 if(res.status===401 && !path.startsWith('/api/v1/auth/')) onUnauthorized()
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
 auth:{
  me:()=>apiFetch<{auth_required:boolean,authenticated:boolean,username?:string}>('/api/v1/auth/me'),
  login:(username:string,password:string)=>apiFetch<{ok:boolean,username?:string}>('/api/v1/auth/login',{method:'POST',body:JSON.stringify({username,password})}),
  logout:()=>apiFetch<{ok:boolean}>('/api/v1/auth/logout',{method:'POST'}),
 },
 stats:()=>apiFetch<any>('/api/v1/stats'),
  leads:(params:Record<string,string>={})=>{
   const clean:Record<string,string>={}; for(const [k,v] of Object.entries(params)) if(v) clean[k]=v
   const q=new URLSearchParams(clean).toString()
   return apiFetch<{data:any[],page:number,per_page:number,total:number,last_page:number}>('/api/v1/leads'+(q?'?'+q:''))
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
 },
companies:{
  search:(params:Record<string,string>)=>{
   const clean:Record<string,string>={}; for(const [k,v] of Object.entries(params)) if(v) clean[k]=v
   const q=new URLSearchParams(clean).toString()
   return apiFetch<{results:any[],notes?:string[]}>('/api/v1/companies/search?'+q)
  },
  suggest:(q:string)=>apiFetch<{suggestions:any[]}>('/api/v1/companies/suggest?q='+encodeURIComponent(q)),
  profile:(params:Record<string,string>)=>{
   const clean:Record<string,string>={}; for(const [k,v] of Object.entries(params)) if(v) clean[k]=v
   return apiFetch<any>('/api/v1/companies/profile?'+new URLSearchParams(clean).toString())
  },
  saveToLead:(body:{company_id?:number,name?:string,domain?:string})=>apiFetch<{lead_id:number,created:boolean}>('/api/v1/companies/save-to-lead',{method:'POST',body:JSON.stringify(body)}),
  exportUrl:()=>'/api/v1/companies/export',
 },
 websites:{
  analyze:(url:string)=>apiFetch<any>('/api/v1/websites/analyze',{method:'POST',body:JSON.stringify({url})}),
  list:(params:Record<string,string>={})=>{
   const clean:Record<string,string>={}; for(const [k,v] of Object.entries(params)) if(v) clean[k]=v
   const q=new URLSearchParams(clean).toString()
   return apiFetch<{data:any[],total:number}>('/api/v1/websites'+(q?'?'+q:''))
  },
  get:(id:number)=>apiFetch<any>('/api/v1/websites/'+id),
 },
 tools:{
  verifyEmail:(email:string)=>apiFetch<any>('/api/v1/tools/verify-email',{method:'POST',body:JSON.stringify({email})}),
  domainAge:(domain:string)=>apiFetch<any>('/api/v1/tools/domain-age?domain='+encodeURIComponent(domain)),
  linkedinFormat:(url:string)=>apiFetch<any>('/api/v1/tools/linkedin-format',{method:'POST',body:JSON.stringify({url})}),
  bulkStart:(type:string,items:string[])=>apiFetch<{id:number,status:string}>('/api/v1/tools/bulk',{method:'POST',body:JSON.stringify({type,items})}),
  bulkStatus:(id:number)=>apiFetch<any>('/api/v1/tools/bulk/'+id),
 }
}
