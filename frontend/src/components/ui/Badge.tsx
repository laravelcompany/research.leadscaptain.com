export function Badge({children,variant='default'}:{children:any,variant?:'default'|'success'|'warning'|'danger'|'info'}){
 const map:any={default:'bg-zinc-100 text-zinc-700', success:'bg-emerald-100 text-emerald-700', warning:'bg-amber-100 text-amber-700', danger:'bg-red-100 text-red-700', info:'bg-blue-100 text-blue-700'}
 return <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${map[variant]}`}>{children}</span>
}
