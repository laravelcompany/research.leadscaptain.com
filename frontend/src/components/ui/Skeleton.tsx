export function Skeleton({className=''}:{className?:string}){ return <div className={`animate-pulse bg-zinc-200 rounded ${className}`} /> }
export function CardSkeleton(){ return <div className="bg-white border rounded-xl p-4 space-y-3"><Skeleton className="h-4 w-24"/><Skeleton className="h-8 w-16"/><Skeleton className="h-3 w-full"/></div>}
