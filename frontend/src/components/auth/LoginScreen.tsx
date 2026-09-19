export function LoginScreen({error}:{error?:string}){
 return <div className="min-h-screen bg-zinc-50 flex items-center justify-center p-4">
  <div className="w-full max-w-sm bg-white border rounded-2xl p-6 shadow-sm space-y-4 text-center">
   <div><h1 className="font-bold text-lg">Autonomous Lead Research</h1><p className="text-xs text-zinc-500 mt-1">Connect your LinkedIn account to continue</p></div>
   {error && <div className="text-left text-xs text-red-600 bg-red-50 border border-red-100 rounded-lg px-3 py-2">{error}</div>}
   <a href="/api/v1/auth/login" className="block w-full bg-[#0A66C2] text-white rounded-xl py-2.5 text-sm font-medium hover:bg-[#004182]">Continue with LinkedIn</a>
   <p className="text-[11px] text-zinc-400">Authentication uses LinkedIn's official OpenID Connect flow.</p>
  </div>
 </div>
}
