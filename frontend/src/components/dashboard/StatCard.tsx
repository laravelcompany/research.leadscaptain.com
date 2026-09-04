import { Skeleton } from '../ui/Skeleton'
export function StatCard({label,value,loading,sub}:{label:string,value:any,loading?:boolean,sub?:string}){
 if(loading) return <div className="bg-white border rounded-2xl p-4 space-y-3"><div className="h-3 w-20 bg-zinc-200 animate-pulse rounded"/><div className="h-7 w-12 bg-zinc-200 animate-pulse rounded"/></div>
 return <div className="bg-white border rounded-2xl p-4"><div className="text-[11px] tracking-widest font-semibold text-zinc-500">{label}</div><div className="text-2xl font-bold mt-2">{value ?? 0}</div>{sub && <div className="text-xs text-zinc-500 mt-1">{sub}</div>}</div>
}
