const BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api";
const SESSION_KEYS = ["access_token", "refresh_token", "user"] as const;

export type User = { ID: string; Name: string; Email: string; Role: string };
export type Branding = {
  app_name: string;
  company_name: string;
  logo_url?: string | null;
  favicon_url?: string | null;
};

export function token() {
  return typeof window !== "undefined"
    ? localStorage.getItem("access_token")
    : null;
}

function clearSession() {
  if (typeof window === "undefined") return;
  SESSION_KEYS.forEach((key) => localStorage.removeItem(key));
}

function saveSession(data: {
  access_token: string;
  refresh_token: string;
  user: User;
}) {
  localStorage.setItem("access_token", data.access_token);
  localStorage.setItem("refresh_token", data.refresh_token);
  localStorage.setItem("user", JSON.stringify(data.user));
}

let refreshRequest: Promise<boolean> | null = null;
async function refreshSession() {
  if (refreshRequest) return refreshRequest;
  refreshRequest = (async () => {
    const refreshToken = localStorage.getItem("refresh_token");
    if (!refreshToken) return false;
    const response = await fetch(`${BASE}/auth/refresh`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refresh_token: refreshToken }),
    });
    if (!response.ok) return false;
    saveSession(await response.json());
    return true;
  })()
    .catch(() => false)
    .finally(() => {
      refreshRequest = null;
    });
  return refreshRequest;
}

async function authenticatedFetch(
  path: string,
  init: RequestInit = {},
  retry = true,
) {
  const headers = new Headers(init.headers);
  const accessToken = token();
  if (accessToken) headers.set("Authorization", `Bearer ${accessToken}`);
  const response = await fetch(`${BASE}${path}`, { ...init, headers });
  if (response.status === 401 && retry && !path.startsWith("/auth/")) {
    if (await refreshSession()) return authenticatedFetch(path, init, false);
    clearSession();
    window.dispatchEvent(new Event("zuldesk:auth-expired"));
  }
  return response;
}

async function errorMessage(response: Response, fallback: string) {
  try {
    return (await response.json()).error || fallback;
  } catch {
    return fallback;
  }
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set("Content-Type", "application/json");
  const response = await authenticatedFetch(path, { ...init, headers });
  if (!response.ok)
    throw new Error(await errorMessage(response, "Ocorreu um erro"));
  if (response.status === 204) return undefined as T;
  return response.json();
}

export async function apiForm<T>(path: string, form: FormData): Promise<T> {
  const response = await authenticatedFetch(path, {
    method: "PATCH",
    body: form,
  });
  if (!response.ok)
    throw new Error(await errorMessage(response, "Ocorreu um erro"));
  return response.json();
}

export async function apiMedia<T>(path: string, form: FormData): Promise<T> {
  const response = await authenticatedFetch(path, {
    method: "POST",
    body: form,
  });
  if (!response.ok)
    throw new Error(
      await errorMessage(response, "Não foi possível enviar a mídia"),
    );
  return response.json();
}

export async function mediaBlob(messageID: string): Promise<Blob> {
  const response = await authenticatedFetch(`/messages/${messageID}/media`);
  if (!response.ok) throw new Error("Mídia indisponível");
  return response.blob();
}

export async function login(email: string, password: string) {
  const data = await api<{
    access_token: string;
    refresh_token: string;
    user: User;
  }>("/auth/login", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });
  saveSession(data);
  return data.user;
}

export async function logout() {
  const refreshToken = localStorage.getItem("refresh_token");
  if (refreshToken) {
    await fetch(`${BASE}/auth/logout`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refresh_token: refreshToken }),
    }).catch(() => {});
  }
  clearSession();
}
