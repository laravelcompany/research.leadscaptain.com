import React from 'react'
export class ErrorBoundary extends React.Component<any,{hasError:boolean,error:any}>{
 state={hasError:false,error:null}
 static getDerivedStateFromError(e:any){ return {hasError:true,error:e}}
 render(){
  if(this.state.hasError) return <div className="p-6 bg-red-50 border border-red-200 rounded-xl"><h3 className="font-bold text-red-800">UI Error</h3><pre className="text-xs whitespace-pre-wrap mt-2">{String(this.state.error)}</pre><button onClick={()=>location.reload()} className="mt-3 bg-red-600 text-white px-3 py-1 rounded">Reload</button></div>
  return this.props.children
 }
}
