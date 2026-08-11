const BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api";
export type User = {ID:string;Name:string;Email:string;Role:string};
export type Branding = {app_name:string;company_name:string;logo_url?:string|null;favicon_url?:string|null};
export function token(){ return typeof window !== "undefined" ? localStorage.getItem("access_token") : null; }
export async function api<T>(path:string, init:RequestInit={}):Promise<T>{
  const headers = new Headers(init.headers); headers.set("Content-Type","application/json"); const t=token(); if(t)headers.set("Authorization",`Bearer ${t}`);
  const res=await fetch(`${BASE}${path}`,{...init,headers}); if(!res.ok){let msg="Ocorreu um erro";try{msg=(await res.json()).error||msg}catch{}throw new Error(msg)}
  if(res.status===204)return undefined as T; return res.json();
}
export async function apiForm<T>(path:string, form:FormData):Promise<T>{const headers=new Headers();const t=token();if(t)headers.set("Authorization",`Bearer ${t}`);const res=await fetch(`${BASE}${path}`,{method:"PATCH",headers,body:form});if(!res.ok){let msg="Ocorreu um erro";try{msg=(await res.json()).error||msg}catch{}throw new Error(msg)}return res.json()}
export async function apiMedia<T>(path:string,form:FormData):Promise<T>{const headers=new Headers();const t=token();if(t)headers.set("Authorization",`Bearer ${t}`);const res=await fetch(`${BASE}${path}`,{method:"POST",headers,body:form});if(!res.ok){let msg="Não foi possível enviar a mídia";try{msg=(await res.json()).error||msg}catch{}throw new Error(msg)}return res.json()}
export async function mediaBlob(messageID:string):Promise<Blob>{const headers=new Headers();const t=token();if(t)headers.set("Authorization",`Bearer ${t}`);const res=await fetch(`${BASE}/messages/${messageID}/media`,{headers});if(!res.ok)throw new Error("Mídia indisponível");return res.blob()}
export async function login(email:string,password:string){const data=await api<{access_token:string;refresh_token:string;user:User}>("/auth/login",{method:"POST",body:JSON.stringify({email,password})});localStorage.setItem("access_token",data.access_token);localStorage.setItem("refresh_token",data.refresh_token);localStorage.setItem("user",JSON.stringify(data.user));return data.user}
export function logout(){localStorage.clear()}
