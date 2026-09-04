export function Button({children, variant='primary', size='md', loading, ...props}:any){
 const base="inline-flex items-center justify-center font-medium rounded-lg transition disabled:opacity-50 disabled:cursor-not-allowed"
 const v:any={primary:'bg-zinc-900 text-white hover:bg-black', secondary:'bg-white border hover:bg-zinc-50', ghost:'hover:bg-zinc-100', danger:'bg-red-600 text-white hover:bg-red-700'}
 const s:any={sm:'px-2.5 py-1 text-xs', md:'px-3.5 py-2 text-sm', lg:'px-4 py-2.5 text-sm'}
 return <button className={`${base} ${v[variant]||v.primary} ${s[size]||s.md}`} disabled={loading||props.disabled} {...props}>{loading ? 'Loading...' : children}</button>
}
