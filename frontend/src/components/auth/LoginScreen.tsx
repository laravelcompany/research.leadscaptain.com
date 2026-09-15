import { useState } from 'react'
import { api } from '../../services/api'

export function LoginScreen({onLogin}:{onLogin:(username:string)=>void}){
 const [username,setUsername]=useState('')
 const [password,setPassword]=useState('')
 const [error,setError]=useState('')
 const [busy,setBusy]=useState(false)

 const submit=async(e:React.FormEvent)=>{
  e.preventDefault()
  if(busy) return
  setError(''); setBusy(true)
  try{
   const r=await api.auth.login(username.trim(),password)
   onLogin(r.username||username.trim())
  }catch(err:any){
   setError(err?.message==='wrong username or password' ? 'Wrong username or password' : (err?.message||'Login failed'))
  }finally{ setBusy(false) }
 }

 return (
  <div className="min-h-screen bg-zinc-50 flex items-center justify-center p-4">
   <form onSubmit={submit} className="w-full max-w-sm bg-white border rounded-2xl p-6 shadow-sm space-y-4">
    <div>
     <h1 className="font-bold text-lg">Autonomous Lead Research</h1>
     <p className="text-xs text-zinc-500 mt-1">Sign in to continue</p>
    </div>
    <div className="space-y-1">
     <label className="text-xs font-medium text-zinc-600" htmlFor="auth-username">Username</label>
     <input id="auth-username" type="text" autoComplete="username" autoFocus required
      value={username} onChange={e=>setUsername(e.target.value)}
      className="w-full border rounded-xl px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-zinc-900/10 focus:border-zinc-400" />
    </div>
    <div className="space-y-1">
     <label className="text-xs font-medium text-zinc-600" htmlFor="auth-password">Password</label>
     <input id="auth-password" type="password" autoComplete="current-password" required
      value={password} onChange={e=>setPassword(e.target.value)}
      className="w-full border rounded-xl px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-zinc-900/10 focus:border-zinc-400" />
    </div>
    {error && <div className="text-xs text-red-600 bg-red-50 border border-red-100 rounded-lg px-3 py-2">{error}</div>}
    <button type="submit" disabled={busy}
     className="w-full bg-zinc-900 text-white rounded-xl py-2 text-sm font-medium hover:bg-zinc-800 disabled:opacity-50">
     {busy?'Signing in...':'Sign in'}
    </button>
   </form>
  </div>
 )
}
