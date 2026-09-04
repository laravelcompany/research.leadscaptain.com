export function EmptyState({title,desc,action}:{title:string,desc?:string,action?:any}){
 return <div className="text-center py-12 px-6"><div className="w-12 h-12 mx-auto bg-zinc-100 rounded-xl grid place-items-center text-xl mb-3">◯</div><h4 className="font-semibold">{title}</h4>{desc && <p className="text-sm text-zinc-500 mt-1 max-w-sm mx-auto">{desc}</p>}{action && <div className="mt-4">{action}</div>}</div>
}
