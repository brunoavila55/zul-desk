"use client";
import {
  FormEvent,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import {
  api,
  apiForm,
  apiMedia,
  mediaBlob,
  Branding,
  login,
  logout,
  token,
  User,
} from "@/lib/api";
import {
  BarChart3,
  Check,
  CheckCheck,
  ChevronDown,
  Clock3,
  File as FileIcon,
  FileText,
  Image as ImageIcon,
  Inbox,
  LayoutDashboard,
  LockKeyhole,
  LogOut,
  Menu,
  MessageCircleMore,
  Mic,
  MoreHorizontal,
  Palette,
  Paperclip,
  Phone,
  Plus,
  RefreshCw,
  Search,
  Send,
  Settings,
  ShieldCheck,
  Smile,
  Smartphone,
  Sticker,
  Tag,
  UserCog,
  Users,
  UserRound,
  Video,
  Wifi,
  WifiOff,
  X,
} from "lucide-react";

type Customer = {
  id: string;
  external_id: string;
  name: string;
  phone: string;
  customer_since: string;
  product: string;
  city: string;
  tags: string[];
  whatsapp_opt_in: boolean;
  opted_out: boolean;
  assigned_user_name?: string;
  has_active_conversation: boolean;
};
type Conversation = {
  id: string;
  status: string;
  customer_id: string;
  customer_name: string;
  phone: string;
  product: string;
  city: string;
  customer_since: string;
  agent_name: string;
  assigned_user_id?: string;
  last_message?: string;
  last_message_at?: string;
  unread: number;
};
type Message = {
  id: string;
  sender_type: string;
  type: string;
  body: string;
  status: string;
  created_at: string;
  user_name?: string;
  error_message?: string;
  media_mime_type?: string;
  media_filename?: string;
  media_size?: number;
  media_ready?: boolean;
};
type Template = {
  id: string;
  whatsapp_account_id?: string;
  name: string;
  status: string;
  content: string;
  category: string;
};
type WhatsAppAccount = {
  id: string;
  name: string;
  business_account_id: string;
  phone_number_id: string;
  display_phone_number: string;
  verified_name?: string;
  quality_rating?: string;
  platform_status?: string;
  onboarding_type: string;
  api_version: string;
  coexistence: boolean;
  active: boolean;
  has_token: boolean;
  last_verified_at?: string;
};
type MetaSettings = {
  callback_url: string;
  verify_token: string;
  app_secret_configured: boolean;
};
type Dashboard = {
  summary: Record<string, number>;
  agents: { id: string; name: string; contacts: number; sales: number }[];
};
type ReportData = {
  days: number;
  summary: Record<string, number>;
  daily: { day: string; started: number; closed: number; sales: number }[];
  agents: {
    id: string;
    name: string;
    contacts: number;
    closed: number;
    sales: number;
    open_now: number;
    conversion_rate: number;
  }[];
  results: { result: string; total: number }[];
};
type TeamUser = {
  id: string;
  name: string;
  email: string;
  role: "ADMIN" | "SUPERVISOR" | "AGENT";
  active: boolean;
  group_ids: string[];
};
type UserGroup = {
  id: string;
  name: string;
  description: string;
  color: string;
  active: boolean;
  user_ids: string[];
  member_count: number;
};
const initials = (s: string) =>
  s
    .split(" ")
    .slice(0, 2)
    .map((x) => x[0])
    .join("");
const formatPhone = (p: string) =>
  p?.replace(/^(55)(\d{2})(\d{5})(\d{4})$/, "+$1 $2 $3-$4") || "";
const time = (d?: string) =>
  d
    ? new Intl.DateTimeFormat("pt-BR", {
        hour: "2-digit",
        minute: "2-digit",
      }).format(new Date(d))
    : "";
const years = (d: string) =>
  d
    ? `${Math.max(0, new Date().getFullYear() - new Date(d).getFullYear())} anos`
    : "—";
const defaultBranding: Branding = {
  app_name: "Zul Desk",
  company_name: "New Life",
};
function BrandLogo({
  branding,
  light = false,
}: {
  branding: Branding;
  light?: boolean;
}) {
  return (
    <div className={`logo ${light ? "light" : ""}`}>
      {branding.logo_url ? (
        <img
          className="custom-logo"
          src={branding.logo_url}
          alt={branding.app_name}
        />
      ) : (
        <span className="brand-mark">
          <MessageCircleMore />
        </span>
      )}
      <span>{branding.app_name}</span>
    </div>
  );
}

export default function Page() {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [branding, setBranding] = useState<Branding>(defaultBranding);
  useEffect(() => {
    const raw = localStorage.getItem("user");
    if (raw && token()) {
      try {
        setUser(JSON.parse(raw));
      } catch {
        void logout();
      }
    }
    api<Branding>("/public/branding")
      .then(setBranding)
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);
  useEffect(() => {
    const expire = () => setUser(null);
    window.addEventListener("zuldesk:auth-expired", expire);
    return () => window.removeEventListener("zuldesk:auth-expired", expire);
  }, []);
  useEffect(() => {
    document.title = branding.app_name;
    let icon = document.querySelector<HTMLLinkElement>('link[rel="icon"]');
    if (branding.favicon_url) {
      if (!icon) {
        icon = document.createElement("link");
        icon.rel = "icon";
        document.head.appendChild(icon);
      }
      icon.href = branding.favicon_url;
    }
  }, [branding]);
  if (loading)
    return (
      <div className="splash">
        <span className="brand-mark">
          <MessageCircleMore />
        </span>
      </div>
    );
  return user ? (
    <App
      user={user}
      branding={branding}
      onBranding={setBranding}
      onLogout={() => {
        logout();
        setUser(null);
      }}
    />
  ) : (
    <Login branding={branding} onLogin={setUser} />
  );
}

function Login({
  branding,
  onLogin,
}: {
  branding: Branding;
  onLogin: (u: User) => void;
}) {
  const demoMode = process.env.NEXT_PUBLIC_DEMO_MODE !== "false";
  const [email, setEmail] = useState(demoMode ? "carlos@newlife.local" : ""),
    [password, setPassword] = useState(demoMode ? "comercial123" : ""),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false);
  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      onLogin(await login(email, password));
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <main className="login-page">
      <section className="login-visual">
        <div className="visual-inner">
          <BrandLogo branding={branding} light />
          <div className="hero-copy">
            <span className="eyebrow">COMERCIAL + WHATSAPP</span>
            <h1>
              Conversas que
              <br />
              viram resultados.
            </h1>
            <p>Seu time, seus clientes e cada oportunidade em um só lugar.</p>
          </div>
          <div className="floating-chat">
            <div className="mini-avatar">JS</div>
            <div>
              <strong>João respondeu</strong>
              <span>“Pode me passar mais informações?”</span>
            </div>
            <span className="online-dot" />
          </div>
        </div>
      </section>
      <section className="login-panel">
        <form onSubmit={submit} className="login-form">
          <div className="mobile-logo">
            <BrandLogo branding={branding} />
          </div>
          <span className="eyebrow green">ÁREA COMERCIAL</span>
          <h2>Bom ter você de volta.</h2>
          <p className="muted">
            Entre para acompanhar seus clientes e conversas.
          </p>
          <label>
            E-mail
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              autoComplete="email"
            />
          </label>
          <label>
            Senha
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="current-password"
            />
          </label>
          {error && <div className="error">{error}</div>}
          <button className="primary full" disabled={busy}>
            {busy ? "Entrando…" : "Entrar na plataforma"}
          </button>
          {demoMode && (
            <div className="demo-hint">
              <LockKeyhole size={15} />
              <span>
                Ambiente de demonstração
                <br />
                <b>Senha: comercial123</b>
              </span>
            </div>
          )}
        </form>
      </section>
    </main>
  );
}

function App({
  user,
  branding,
  onBranding,
  onLogout,
}: {
  user: User;
  branding: Branding;
  onBranding: (b: Branding) => void;
  onLogout: () => void;
}) {
  const [view, setView] = useState("inbox"),
    [mobileNav, setMobileNav] = useState(false),
    [connected, setConnected] = useState(false),
    [tick, setTick] = useState(0);
  useEffect(() => {
    const base = (process.env.NEXT_PUBLIC_API_URL || location.origin + "/api")
      .replace(/^http/, "ws")
      .replace(/\/api$/, "");
    let ws: WebSocket | null = null;
    let retry: ReturnType<typeof setTimeout> | null = null;
    let stopped = false;
    const connect = () => {
      if (stopped) return;
      ws = new WebSocket(`${base}/ws?token=${token()}`);
      ws.onopen = () => setConnected(true);
      ws.onmessage = () => setTick((x) => x + 1);
      ws.onclose = () => {
        setConnected(false);
        if (!stopped) {
          void api<User>("/auth/me").finally(() => {
            if (!stopped) retry = setTimeout(connect, 2000);
          });
        }
      };
    };
    connect();
    return () => {
      stopped = true;
      if (retry) clearTimeout(retry);
      ws?.close();
    };
  }, []);
  const nav = [
    { id: "dashboard", label: "Visão geral", icon: LayoutDashboard },
    { id: "inbox", label: "Conversas", icon: Inbox },
    { id: "customers", label: "Clientes", icon: Users },
  ];
  return (
    <div className="shell">
      <aside className={`sidebar ${mobileNav ? "open" : ""}`}>
        <div className="sidebar-brand">
          <BrandLogo branding={branding} />
          <button
            className="icon-btn close-nav"
            onClick={() => setMobileNav(false)}
          >
            <X />
          </button>
        </div>
        <div className="workspace">
          <span className="avatar company">
            {initials(branding.company_name)}
          </span>
          <div>
            <b>{branding.company_name}</b>
            <span>Comercial</span>
          </div>
          <ChevronDown size={16} />
        </div>
        <nav>
          <small>MENU</small>
          {nav.map((n) => (
            <button
              key={n.id}
              className={view === n.id ? "active" : ""}
              onClick={() => {
                setView(n.id);
                setMobileNav(false);
              }}
            >
              <n.icon size={19} />
              <span>{n.label}</span>
              {n.id === "inbox" && <em>3</em>}
            </button>
          ))}
          {user.Role !== "AGENT" && (
            <>
              <small>GESTÃO</small>
              <button
                className={view === "reports" ? "active" : ""}
                onClick={() => {
                  setView("reports");
                  setMobileNav(false);
                }}
              >
                <BarChart3 size={19} />
                <span>Relatórios</span>
              </button>
              <button
                className={view === "settings" ? "active" : ""}
                onClick={() => setView("settings")}
              >
                <Settings size={19} />
                <span>Configurações</span>
              </button>
            </>
          )}
        </nav>
        <div className="sidebar-foot">
          <div className="profile">
            <span className="avatar">{initials(user.Name)}</span>
            <div>
              <b>{user.Name}</b>
              <span>{user.Role === "AGENT" ? "Usuário" : user.Role}</span>
            </div>
            <button className="icon-btn" onClick={onLogout} title="Sair">
              <LogOut size={17} />
            </button>
          </div>
        </div>
      </aside>
      <main className="main">
        <header className="topbar">
          <button
            className="icon-btn menu-btn"
            onClick={() => setMobileNav(true)}
          >
            <Menu />
          </button>
          <div>
            <h2>
              {nav.find((n) => n.id === view)?.label ||
                (view === "reports" ? "Relatórios" : "Configurações")}
            </h2>
            <p>
              {view === "inbox"
                ? "Acompanhe e responda seus atendimentos"
                : view === "customers"
                  ? "Encontre e conheça sua base de clientes"
                  : view === "dashboard"
                    ? "Seu desempenho comercial hoje"
                    : view === "reports"
                      ? "Analise volume, conversão e desempenho da equipe"
                      : "Gerencie integrações, equipe e identidade do aplicativo"}
            </p>
          </div>
          <div className={`connection ${connected ? "ok" : ""}`}>
            {connected ? <Wifi size={15} /> : <WifiOff size={15} />}{" "}
            {connected ? "Tempo real ativo" : "Reconectando"}
          </div>
        </header>
        <div className="content">
          {view === "inbox" && <InboxView tick={tick} user={user} />}{" "}
          {view === "customers" && (
            <CustomersView onOpenInbox={() => setView("inbox")} />
          )}{" "}
          {view === "dashboard" && (
            <DashboardView onReports={() => setView("reports")} />
          )}{" "}
          {view === "reports" && <ReportsView />}{" "}
          {view === "settings" && (
            <SettingsView
              user={user}
              branding={branding}
              onBranding={onBranding}
            />
          )}
        </div>
      </main>
    </div>
  );
}

function InboxView({ tick, user }: { tick: number; user: User }) {
  const [list, setList] = useState<Conversation[]>([]),
    [selected, setSelected] = useState<Conversation | null>(null),
    [messages, setMessages] = useState<Message[]>([]),
    [query, setQuery] = useState(""),
    [draft, setDraft] = useState(""),
    [error, setError] = useState(""),
    [success, setSuccess] = useState(""),
    [action, setAction] = useState<"note" | "transfer" | "close" | null>(null),
    [actionText, setActionText] = useState(""),
    [result, setResult] = useState("SALE"),
    [assignee, setAssignee] = useState(""),
    [team, setTeam] = useState<TeamUser[]>([]),
    [busy, setBusy] = useState(false),
    [attachOpen, setAttachOpen] = useState(false),
    [emojiOpen, setEmojiOpen] = useState(false),
    [recording, setRecording] = useState(false),
    [uploading, setUploading] = useState(false),
    [scope, setScope] = useState<"mine" | "all">("mine");
  const recorderRef = useRef<MediaRecorder | null>(null),
    chunksRef = useRef<Blob[]>([]);
  const load = useCallback(async () => {
    try {
      const x = await api<{ items: Conversation[] }>(
        `/conversations?scope=${scope}`,
      );
      const active = x.items
        .filter((c) => c.status !== "CLOSED")
        .map((c) =>
          scope === "all"
            ? {
                ...c,
                last_message: `${c.agent_name} · ${c.last_message || "Atendimento iniciado"}`,
              }
            : c,
        );
      setList(active);
      setSelected((current) =>
        current
          ? active.find((c) => c.id === current.id) || null
          : active[0] || null,
      );
    } catch (e) {
      setError((e as Error).message);
    }
  }, [scope]);
  useEffect(() => {
    void load();
  }, [tick, load]);
  useEffect(() => {
    const tabs = document.querySelector(".inbox-grid .tabs");
    if (!tabs) return;
    const buttons = tabs.querySelectorAll("button");
    const mine = buttons[0],
      teamButton = buttons[1];
    const mineClick = () => setScope("mine"),
      teamClick = () => setScope("all");
    mine.textContent = "Minhas";
    mine.classList.toggle("active", scope === "mine");
    mine.addEventListener("click", mineClick);
    if (teamButton) {
      teamButton.textContent = "Todas da equipe";
      teamButton.classList.toggle("active", scope === "all");
      teamButton.style.display = user.Role === "AGENT" ? "none" : "";
      teamButton.addEventListener("click", teamClick);
    }
    return () => {
      mine.removeEventListener("click", mineClick);
      teamButton?.removeEventListener("click", teamClick);
    };
  }, [scope, user.Role]);
  useEffect(() => {
    if (selected)
      api<{ items: Message[] }>(`/conversations/${selected.id}/messages`)
        .then((x) => setMessages(x.items))
        .catch((e) => setError(e.message));
    else setMessages([]);
  }, [selected, tick]);
  async function send() {
    if (
      !draft.trim() ||
      !selected ||
      selected.status === "CLOSED" ||
      !ownsSelected
    )
      return;
    try {
      await api(`/conversations/${selected.id}/messages`, {
        method: "POST",
        body: JSON.stringify({ body: draft }),
      });
      setDraft("");
      setTimeout(
        () =>
          api<{ items: Message[] }>(
            `/conversations/${selected.id}/messages`,
          ).then((x) => setMessages(x.items)),
        250,
      );
    } catch (e) {
      setError((e as Error).message);
    }
  }
  async function upload(file: File, kind?: string) {
    if (!selected || !ownsSelected) return;
    setUploading(true);
    setAttachOpen(false);
    setError("");
    const form = new FormData();
    form.set("file", file);
    if (kind) form.set("kind", kind);
    if (draft.trim()) form.set("caption", draft.trim());
    try {
      await apiMedia(`/conversations/${selected.id}/media`, form);
      setDraft("");
      setTimeout(
        () =>
          api<{ items: Message[] }>(
            `/conversations/${selected.id}/messages`,
          ).then((x) => setMessages(x.items)),
        300,
      );
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setUploading(false);
    }
  }
  async function startRecording() {
    if (!ownsSelected) return;
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      const preferred = MediaRecorder.isTypeSupported("audio/ogg;codecs=opus")
        ? "audio/ogg;codecs=opus"
        : "audio/webm;codecs=opus";
      const recorder = new MediaRecorder(stream, { mimeType: preferred });
      chunksRef.current = [];
      recorder.ondataavailable = (e) => {
        if (e.data.size) chunksRef.current.push(e.data);
      };
      recorder.onstop = () => {
        stream.getTracks().forEach((t) => t.stop());
        const blob = new Blob(chunksRef.current, { type: recorder.mimeType });
        void upload(
          new File(
            [blob],
            `audio-${Date.now()}.${recorder.mimeType.includes("ogg") ? "ogg" : "webm"}`,
            { type: recorder.mimeType },
          ),
          "audio",
        );
      };
      recorder.start();
      recorderRef.current = recorder;
      setRecording(true);
    } catch {
      setError("Permita o uso do microfone para gravar áudio.");
    }
  }
  function stopRecording() {
    recorderRef.current?.stop();
    setRecording(false);
  }
  async function openAction(kind: "note" | "transfer" | "close") {
    setAction(kind);
    setActionText("");
    setError("");
    if (kind === "transfer" && !team.length) {
      try {
        const x = await api<{ items: TeamUser[] }>("/users");
        setTeam(x.items.filter((x) => x.active));
        setAssignee(
          x.items.find((x) => x.active && x.id !== selected?.assigned_user_id)
            ?.id || "",
        );
      } catch (e) {
        setError((e as Error).message);
      }
    }
  }
  async function submitAction() {
    if (!selected || !action) return;
    setBusy(true);
    setError("");
    try {
      if (action === "note")
        await api(`/conversations/${selected.id}/notes`, {
          method: "POST",
          body: JSON.stringify({ content: actionText }),
        });
      if (action === "transfer")
        await api(`/conversations/${selected.id}/assign`, {
          method: "POST",
          body: JSON.stringify({ user_id: assignee }),
        });
      if (action === "close")
        await api(`/conversations/${selected.id}/close`, {
          method: "POST",
          body: JSON.stringify({ result, note: actionText }),
        });
      setSuccess(
        action === "note"
          ? "Nota adicionada ao histórico."
          : action === "transfer"
            ? "Atendimento transferido com sucesso."
            : "Atendimento encerrado.",
      );
      setAction(null);
      await load();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  const filtered = list.filter((c) =>
    c.customer_name.toLowerCase().includes(query.toLowerCase()),
  );
  const ownsSelected = selected?.assigned_user_id === user.ID;
  useEffect(() => {
    const composer = document.querySelector(".inbox-grid .composer");
    if (!composer) return;
    const field = composer.querySelector("textarea");
    const controls = composer.querySelectorAll("button");
    const locked = !!selected && !ownsSelected;
    if (field) {
      field.disabled = locked || selected?.status === "CLOSED";
      if (locked)
        field.placeholder = `Somente ${selected?.agent_name} pode responder · transfira para assumir`;
    }
    controls.forEach((button) => {
      button.disabled = locked || selected?.status === "CLOSED";
    });
  }, [ownsSelected, selected?.id, selected?.agent_name, selected?.status]);
  return (
    <>
      <div className="inbox-grid">
        <section className="conversation-list">
          <div className="list-head">
            <div className="tabs">
              <button className="active">Minhas</button>
              <button>Não atribuídas</button>
            </div>
            <div className="search">
              <Search size={17} />
              <input
                placeholder="Buscar conversa"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
              />
            </div>
            <div className="filter-row">
              <button className="filter active">
                Todas <b>{list.length}</b>
              </button>
              <button className="filter">Aguardando cliente</button>
              <button className="filter">Aguardando você</button>
            </div>
          </div>
          <div className="threads">
            {filtered.map((c) => (
              <button
                key={c.id}
                className={`thread ${selected?.id === c.id ? "selected" : ""}`}
                onClick={() => {
                  setSelected(c);
                  setSuccess("");
                }}
              >
                <span className="avatar">{initials(c.customer_name)}</span>
                <div className="thread-copy">
                  <div>
                    <strong>{c.customer_name}</strong>
                    <time>{time(c.last_message_at)}</time>
                  </div>
                  <p>{c.last_message || "Atendimento iniciado"}</p>
                  <span
                    className={`state ${c.status === "WAITING_AGENT" ? "attention" : ""}`}
                  >
                    {c.status === "WAITING_AGENT"
                      ? "Aguardando você"
                      : "Aguardando cliente"}
                  </span>
                </div>
                {Number(c.unread) > 0 && <em className="unread">{c.unread}</em>}
              </button>
            ))}
            {!filtered.length && (
              <Empty
                icon={<MessageCircleMore />}
                title="Nenhuma conversa"
                text="Inicie um contato pela área de clientes."
              />
            )}
          </div>
        </section>
        {selected ? (
          <>
            <section className="chat">
              <div className="chat-head">
                <span className="avatar">
                  {initials(selected.customer_name)}
                </span>
                <div>
                  <strong>{selected.customer_name}</strong>
                  <span>
                    {formatPhone(selected.phone)} · <i /> WhatsApp
                  </span>
                </div>
                <button className="icon-btn">
                  <Search size={18} />
                </button>
                <button className="icon-btn">
                  <MoreHorizontal size={20} />
                </button>
              </div>
              <div className="messages">
                <div className="day-pill">Hoje</div>
                {messages.map((m) => (
                  <div
                    key={m.id}
                    className={`bubble-wrap ${m.sender_type === "AGENT" ? "out" : "in"}`}
                  >
                    <span className="sender">
                      {m.sender_type === "AGENT"
                        ? m.user_name || "Você"
                        : selected.customer_name.split(" ")[0]}
                    </span>
                    <div
                      className={`bubble ${m.type !== "TEXT" && m.type !== "TEMPLATE" ? "media-bubble" : ""}`}
                    >
                      <MessageContent message={m} />
                      <span className="bubble-time">
                        {time(m.created_at)}{" "}
                        {m.sender_type === "AGENT" &&
                          (m.status === "READ" ? (
                            <CheckCheck className="read" />
                          ) : m.status === "DELIVERED" ? (
                            <CheckCheck />
                          ) : (
                            <Check />
                          ))}
                      </span>
                    </div>
                    {m.error_message && (
                      <span className="msg-error">{m.error_message}</span>
                    )}
                  </div>
                ))}
                {!messages.length && (
                  <div className="empty-chat">
                    <ShieldCheck />
                    <span>Conversa protegida e registrada</span>
                  </div>
                )}
              </div>
              {success && (
                <div className="success inline-success">
                  <Check />
                  {success}
                </div>
              )}
              {error && !action && (
                <div className="inline-error">
                  {error}
                  <button onClick={() => setError("")}>
                    <X />
                  </button>
                </div>
              )}
              <div className="composer-wrap">
                {attachOpen && (
                  <div className="attach-menu">
                    <label>
                      <ImageIcon />
                      <span>
                        Imagem
                        <input
                          type="file"
                          accept="image/jpeg,image/png,image/webp"
                          onChange={(e) =>
                            e.target.files?.[0] &&
                            upload(e.target.files[0], "image")
                          }
                        />
                      </span>
                    </label>
                    <label>
                      <Video />
                      <span>
                        Vídeo
                        <input
                          type="file"
                          accept="video/mp4"
                          onChange={(e) =>
                            e.target.files?.[0] &&
                            upload(e.target.files[0], "video")
                          }
                        />
                      </span>
                    </label>
                    <label>
                      <FileIcon />
                      <span>
                        Documento
                        <input
                          type="file"
                          onChange={(e) =>
                            e.target.files?.[0] &&
                            upload(e.target.files[0], "document")
                          }
                        />
                      </span>
                    </label>
                    <label>
                      <Sticker />
                      <span>
                        Figurinha
                        <input
                          type="file"
                          accept="image/webp"
                          onChange={(e) =>
                            e.target.files?.[0] &&
                            upload(e.target.files[0], "sticker")
                          }
                        />
                      </span>
                    </label>
                  </div>
                )}
                {emojiOpen && (
                  <div className="emoji-picker">
                    {[
                      "😀",
                      "😂",
                      "🥰",
                      "😍",
                      "🤩",
                      "😊",
                      "🙏",
                      "👍",
                      "👏",
                      "🎉",
                      "❤️",
                      "🔥",
                      "✅",
                      "👀",
                      "🤝",
                      "💬",
                      "📞",
                      "🚀",
                    ].map((e) => (
                      <button
                        key={e}
                        onClick={() => {
                          setDraft((d) => d + e);
                          setEmojiOpen(false);
                        }}
                      >
                        {e}
                      </button>
                    ))}
                  </div>
                )}
                <div className="composer">
                  <button
                    className={`icon-btn ${attachOpen ? "active" : ""}`}
                    onClick={() => {
                      setAttachOpen((x) => !x);
                      setEmojiOpen(false);
                    }}
                    disabled={uploading}
                  >
                    <Paperclip />
                  </button>
                  <button
                    className={`icon-btn ${emojiOpen ? "active" : ""}`}
                    onClick={() => {
                      setEmojiOpen((x) => !x);
                      setAttachOpen(false);
                    }}
                  >
                    <Smile />
                  </button>
                  <textarea
                    disabled={selected.status === "CLOSED"}
                    placeholder={
                      recording
                        ? "Gravando áudio…"
                        : uploading
                          ? "Enviando mídia…"
                          : selected.status === "CLOSED"
                            ? "Atendimento encerrado"
                            : "Digite uma mensagem…"
                    }
                    value={draft}
                    onChange={(e) => setDraft(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" && !e.shiftKey) {
                        e.preventDefault();
                        send();
                      }
                    }}
                  />
                  {draft.trim() ? (
                    <button
                      className="send-btn"
                      onClick={send}
                      disabled={selected.status === "CLOSED"}
                    >
                      <Send />
                    </button>
                  ) : (
                    <button
                      className={`send-btn mic-btn ${recording ? "recording" : ""}`}
                      onClick={recording ? stopRecording : startRecording}
                      disabled={selected.status === "CLOSED" || uploading}
                    >
                      <Mic />
                    </button>
                  )}
                </div>
              </div>
            </section>
            <CustomerPanel
              c={selected}
              canTransfer={user.Role !== "AGENT"}
              onAction={openAction}
            />
          </>
        ) : (
          <section className="chat blank">
            <Empty
              icon={<MessageCircleMore />}
              title="Suas conversas aparecerão aqui"
              text="Selecione um atendimento para começar."
            />
          </section>
        )}
      </div>
      {action && selected && (
        <div className="modal-backdrop">
          <div className="modal action-modal">
            <div className="modal-head">
              <div>
                <span className="eyebrow green">ATENDIMENTO</span>
                <h3>
                  {action === "note"
                    ? "Adicionar nota"
                    : action === "transfer"
                      ? "Transferir conversa"
                      : "Encerrar atendimento"}
                </h3>
              </div>
              <button className="icon-btn" onClick={() => setAction(null)}>
                <X />
              </button>
            </div>
            {error && <div className="error banner">{error}</div>}
            {action === "transfer" ? (
              <label>
                Novo responsável
                <select
                  value={assignee}
                  onChange={(e) => setAssignee(e.target.value)}
                >
                  {team
                    .filter((x) => x.id !== selected.assigned_user_id)
                    .map((x) => (
                      <option key={x.id} value={x.id}>
                        {x.name} · {roleLabel(x.role)}
                      </option>
                    ))}
                </select>
              </label>
            ) : (
              <>
                {action === "close" && (
                  <label>
                    Resultado
                    <select
                      value={result}
                      onChange={(e) => setResult(e.target.value)}
                    >
                      <option value="SALE">Venda realizada</option>
                      <option value="INTERESTED">Cliente interessado</option>
                      <option value="CALLBACK">Retornar depois</option>
                      <option value="NO_INTEREST">Sem interesse</option>
                      <option value="NO_RESPONSE">Sem resposta</option>
                      <option value="INVALID_NUMBER">Número inválido</option>
                      <option value="OTHER">Outro</option>
                    </select>
                  </label>
                )}
                <label>
                  {action === "note" ? "Nota interna" : "Observação final"}
                  <textarea
                    className="action-text"
                    autoFocus
                    value={actionText}
                    onChange={(e) => setActionText(e.target.value)}
                    placeholder={
                      action === "note"
                        ? "Registre uma informação importante para a equipe…"
                        : "Contexto opcional sobre o encerramento…"
                    }
                  />
                </label>
              </>
            )}
            <div className="modal-foot">
              <span>
                <ShieldCheck /> A ação ficará registrada no histórico.
              </span>
              <button
                className="primary"
                onClick={submitAction}
                disabled={
                  busy ||
                  (action === "note" && !actionText.trim()) ||
                  (action === "transfer" && !assignee)
                }
              >
                <Check />
                {busy ? "Salvando…" : "Confirmar"}
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}

function MessageContent({ message: m }: { message: Message }) {
  const [url, setURL] = useState(""),
    [mediaError, setMediaError] = useState("");
  useEffect(() => {
    let active = true,
      objectURL = "";
    if (m.type === "TEXT" || m.type === "TEMPLATE" || !m.media_ready) return;
    mediaBlob(m.id)
      .then((blob) => {
        if (active) {
          objectURL = URL.createObjectURL(blob);
          setURL(objectURL);
        }
      })
      .catch((e) => active && setMediaError(e.message));
    return () => {
      active = false;
      if (objectURL) URL.revokeObjectURL(objectURL);
    };
  }, [m.id, m.type, m.media_ready]);
  if (m.type === "TEXT" || m.type === "TEMPLATE") return <>{m.body}</>;
  if (!m.media_ready)
    return (
      <div className="media-loading">
        <RefreshCw /> Preparando {m.body.toLowerCase()}…
      </div>
    );
  if (mediaError)
    return <div className="media-loading error-text">{mediaError}</div>;
  if (!url)
    return (
      <div className="media-loading">
        <RefreshCw /> Carregando mídia…
      </div>
    );
  if (m.type === "IMAGE" || m.type === "STICKER")
    return (
      <div className={m.type === "STICKER" ? "sticker-media" : "image-media"}>
        <img src={url} alt={m.body || m.media_filename || "Mídia"} />
        {m.body && !["Imagem", "Figurinha"].includes(m.body) && <p>{m.body}</p>}
      </div>
    );
  if (m.type === "AUDIO")
    return (
      <div className="audio-media">
        <Mic />
        <audio controls preload="metadata" src={url} />
      </div>
    );
  if (m.type === "VIDEO")
    return (
      <div className="video-media">
        <video controls preload="metadata" src={url} />
        {m.body !== "Vídeo" && <p>{m.body}</p>}
      </div>
    );
  return (
    <a
      className="document-media"
      href={url}
      download={m.media_filename || "documento"}
    >
      <span>
        <FileIcon />
      </span>
      <div>
        <b>{m.media_filename || m.body || "Documento"}</b>
        <small>
          {m.media_size
            ? `${(Number(m.media_size) / 1024 / 1024).toFixed(1)} MB`
            : "Toque para baixar"}
        </small>
      </div>
    </a>
  );
}

function CustomerPanel({
  c,
  onAction,
}: {
  c: Conversation;
  canTransfer: boolean;
  onAction: (kind: "note" | "transfer" | "close") => void;
}) {
  return (
    <aside className="customer-panel">
      <div className="customer-hero">
        <span className="avatar large">{initials(c.customer_name)}</span>
        <h3>{c.customer_name}</h3>
        <p>{formatPhone(c.phone)}</p>
        <span className="opt">
          <Check /> Contato autorizado
        </span>
      </div>
      <div className="panel-section">
        <small>DADOS DO CLIENTE</small>
        <Info
          label="Cliente desde"
          value={
            c.customer_since
              ? new Date(c.customer_since).toLocaleDateString("pt-BR")
              : "—"
          }
        />
        <Info label="Tempo de casa" value={years(c.customer_since)} />
        <Info label="Plano atual" value={c.product} />
        <Info label="Cidade" value={c.city} />
      </div>
      <div className="panel-section">
        <small>ATENDIMENTO</small>
        <Info label="Responsável" value={c.agent_name} />
        <Info
          label="Status"
          value={
            c.status === "WAITING_AGENT"
              ? "Aguardando você"
              : "Aguardando cliente"
          }
        />
      </div>
      <div className="panel-actions">
        <button onClick={() => onAction("note")}>
          <Plus /> Adicionar nota
        </button>
        <button onClick={() => onAction("transfer")}>
          <UserRound /> Transferir atendimento
        </button>
        <button onClick={() => onAction("close")}>
          <Check /> Encerrar atendimento
        </button>
      </div>
    </aside>
  );
}
function Info({ label, value }: { label: string; value?: string }) {
  return (
    <div className="info">
      <span>{label}</span>
      <b>{value || "—"}</b>
    </div>
  );
}

function CustomersView({ onOpenInbox }: { onOpenInbox: () => void }) {
  const [items, setItems] = useState<Customer[]>([]),
    [q, setQ] = useState(""),
    [selected, setSelected] = useState<Customer | null>(null),
    [templates, setTemplates] = useState<Template[]>([]),
    [accounts, setAccounts] = useState<WhatsAppAccount[]>([]),
    [accountID, setAccountID] = useState(""),
    [modal, setModal] = useState(false),
    [createModal, setCreateModal] = useState(false),
    [saving, setSaving] = useState(false),
    [choice, setChoice] = useState(""),
    [preview, setPreview] = useState(""),
    [message, setMessage] = useState(""),
    [error, setError] = useState("");
  const emptyCustomer = {
    external_id: "",
    name: "",
    phone: "",
    document: "",
    customer_since: "",
    product: "",
    city: "",
    whatsapp_opt_in: false,
    opt_in_source: "Cadastro manual no Zul Desk",
  };
  const [customerForm, setCustomerForm] = useState(emptyCustomer);
  useEffect(() => {
    const t = setTimeout(
      () =>
        api<{ items: Customer[] }>(
          `/customers?q=${encodeURIComponent(q)}`,
        ).then((x) => setItems(x.items)),
      250,
    );
    return () => clearTimeout(t);
  }, [q]);
  async function begin(c: Customer) {
    setSelected(c);
    setError("");
    try {
      const [a, t] = await Promise.all([
        api<{ items: WhatsAppAccount[] }>("/whatsapp/accounts"),
        api<{ items: Template[] }>("/templates"),
      ]);
      const active = a.items.filter((x) => x.active);
      setAccounts(active);
      setTemplates(t.items.filter((x) => x.status === "APPROVED"));
      const account = active[0];
      if (!account) throw new Error("Nenhum número do WhatsApp está ativo");
      setAccountID(account.id);
      const first = t.items.find(
        (x) =>
          x.status === "APPROVED" &&
          (!x.whatsapp_account_id || x.whatsapp_account_id === account.id),
      );
      if (first) {
        setChoice(first.name);
        setPreview(renderTemplate(first, c));
      }
      setModal(true);
    } catch (e) {
      setError((e as Error).message);
    }
  }
  async function createCustomer(e: FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError("");
    setMessage("");
    try {
      await api("/customers", {
        method: "POST",
        body: JSON.stringify(customerForm),
      });
      const result = await api<{ items: Customer[] }>(
        `/customers?q=${encodeURIComponent(q)}`,
      );
      setItems(result.items);
      setCustomerForm(emptyCustomer);
      setCreateModal(false);
      setMessage("Cliente cadastrado com sucesso.");
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setSaving(false);
    }
  }
  function renderTemplate(t: Template, c: Customer) {
    return t.content
      .replace("{{1}}", c.name.split(" ")[0])
      .replace("{{2}}", "Carlos")
      .replace("{{3}}", years(c.customer_since));
  }
  async function submit() {
    if (!selected) return;
    try {
      await api("/conversations", {
        method: "POST",
        body: JSON.stringify({
          customer_id: selected.id,
          whatsapp_account_id: accountID,
          template_name: choice,
          body: preview,
        }),
      });
      setModal(false);
      onOpenInbox();
    } catch (e) {
      setError((e as Error).message);
    }
  }
  return (
    <>
      <div className="page-tools">
        <div className="big-search">
          <Search />
          <input
            placeholder="Busque por nome, telefone, código ou CPF/CNPJ"
            value={q}
            onChange={(e) => setQ(e.target.value)}
          />
        </div>
        <button className="secondary" onClick={() => setCreateModal(true)}>
          <Plus /> Novo cliente
        </button>
      </div>
      <div className="filter-bar">
        <button>
          Tempo como cliente <ChevronDown />
        </button>
        <button>
          Plano <ChevronDown />
        </button>
        <button>
          Cidade <ChevronDown />
        </button>
        <button>
          Meus clientes <ChevronDown />
        </button>
      </div>
      {error && <div className="error banner">{error}</div>}
      {message && (
        <div className="success banner">
          <Check /> {message}
        </div>
      )}
      <section className="customer-table">
        <div className="table-head">
          <span>CLIENTE</span>
          <span>CLIENTE DESDE</span>
          <span>PLANO</span>
          <span>CIDADE</span>
          <span>WHATSAPP</span>
          <span />
        </div>
        {items.map((c) => (
          <div className="customer-row" key={c.id}>
            <div className="person">
              <span className="avatar">{initials(c.name)}</span>
              <div>
                <strong>{c.name}</strong>
                <span>
                  #{c.external_id} · {formatPhone(c.phone)}
                </span>
              </div>
            </div>
            <span>
              {c.customer_since
                ? new Date(c.customer_since).toLocaleDateString("pt-BR")
                : "—"}
              <small>{years(c.customer_since)}</small>
            </span>
            <b>{c.product}</b>
            <span>{c.city}</span>
            <span
              className={`permission ${c.whatsapp_opt_in && !c.opted_out ? "yes" : "no"}`}
            >
              {c.whatsapp_opt_in && !c.opted_out ? (
                <>
                  <Check /> Autorizado
                </>
              ) : (
                <>
                  <X /> Bloqueado
                </>
              )}
            </span>
            <button
              className="outline"
              disabled={
                !c.whatsapp_opt_in || c.opted_out || c.has_active_conversation
              }
              onClick={() => begin(c)}
            >
              {c.has_active_conversation
                ? "Em atendimento"
                : "Iniciar conversa"}
            </button>
          </div>
        ))}
        {!items.length && (
          <Empty
            icon={<Users />}
            title="Nenhum cliente encontrado"
            text="Tente buscar por outro termo."
          />
        )}
      </section>
      {createModal && (
        <div className="modal-backdrop">
          <form className="modal customer-modal" onSubmit={createCustomer}>
            <div className="modal-head">
              <div>
                <span className="eyebrow green">NOVO CLIENTE</span>
                <h3>Cadastrar cliente</h3>
              </div>
              <button
                type="button"
                className="icon-btn"
                onClick={() => setCreateModal(false)}
              >
                <X />
              </button>
            </div>
            <div className="form-grid">
              <label className="wide">
                Nome completo
                <input
                  required
                  autoFocus
                  value={customerForm.name}
                  onChange={(e) =>
                    setCustomerForm({ ...customerForm, name: e.target.value })
                  }
                  placeholder="Ex.: João da Silva"
                />
              </label>
              <label>
                WhatsApp
                <input
                  required
                  inputMode="tel"
                  value={customerForm.phone}
                  onChange={(e) =>
                    setCustomerForm({ ...customerForm, phone: e.target.value })
                  }
                  placeholder="Ex.: 55 11 99999-9999"
                />
              </label>
              <label>
                Código do cliente
                <input
                  value={customerForm.external_id}
                  onChange={(e) =>
                    setCustomerForm({
                      ...customerForm,
                      external_id: e.target.value,
                    })
                  }
                  placeholder="Código no ERP (opcional)"
                />
              </label>
              <label>
                CPF/CNPJ
                <input
                  value={customerForm.document}
                  onChange={(e) =>
                    setCustomerForm({
                      ...customerForm,
                      document: e.target.value,
                    })
                  }
                  placeholder="Opcional"
                />
              </label>
              <label>
                Cliente desde
                <input
                  type="date"
                  value={customerForm.customer_since}
                  onChange={(e) =>
                    setCustomerForm({
                      ...customerForm,
                      customer_since: e.target.value,
                    })
                  }
                />
              </label>
              <label>
                Plano ou produto
                <input
                  value={customerForm.product}
                  onChange={(e) =>
                    setCustomerForm({
                      ...customerForm,
                      product: e.target.value,
                    })
                  }
                  placeholder="Ex.: Fibra 500 Mbps"
                />
              </label>
              <label>
                Cidade
                <input
                  value={customerForm.city}
                  onChange={(e) =>
                    setCustomerForm({ ...customerForm, city: e.target.value })
                  }
                  placeholder="Ex.: São Gabriel"
                />
              </label>
              <label className="check-label wide">
                <input
                  type="checkbox"
                  checked={customerForm.whatsapp_opt_in}
                  onChange={(e) =>
                    setCustomerForm({
                      ...customerForm,
                      whatsapp_opt_in: e.target.checked,
                    })
                  }
                />
                <span>
                  <b>Cliente autorizou contato pelo WhatsApp</b>
                  <small>
                    Marque somente quando houver consentimento registrado.
                  </small>
                </span>
              </label>
              {customerForm.whatsapp_opt_in && (
                <label className="wide">
                  Origem da autorização
                  <input
                    required
                    value={customerForm.opt_in_source}
                    onChange={(e) =>
                      setCustomerForm({
                        ...customerForm,
                        opt_in_source: e.target.value,
                      })
                    }
                    placeholder="Ex.: Contrato, ligação ou formulário"
                  />
                </label>
              )}
            </div>
            <div className="modal-foot">
              <span>
                <ShieldCheck /> Consentimento e origem ficam registrados
              </span>
              <button className="primary" disabled={saving}>
                <Plus /> {saving ? "Cadastrando…" : "Cadastrar cliente"}
              </button>
            </div>
          </form>
        </div>
      )}
      {modal && selected && (
        <div className="modal-backdrop">
          <div className="modal">
            <div className="modal-head">
              <div>
                <span className="eyebrow green">NOVA CONVERSA</span>
                <h3>Falar com {selected.name}</h3>
              </div>
              <button className="icon-btn" onClick={() => setModal(false)}>
                <X />
              </button>
            </div>
            <div className="notice">
              <Clock3 />
              <div>
                <b>Fora da janela de atendimento</b>
                <span>
                  A conversa precisa começar com um template aprovado pela Meta.
                </span>
              </div>
            </div>
            <label>
              Número de saída
              <select
                value={accountID}
                onChange={(e) => {
                  const id = e.target.value;
                  setAccountID(id);
                  const available = templates.find(
                    (x) =>
                      x.status === "APPROVED" &&
                      (!x.whatsapp_account_id || x.whatsapp_account_id === id),
                  );
                  setChoice(available?.name || "");
                  if (available)
                    setPreview(renderTemplate(available, selected));
                }}
              >
                {accounts.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.name} · {a.display_phone_number}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Template
              <select
                value={choice}
                onChange={(e) => {
                  setChoice(e.target.value);
                  const t = templates.find((x) => x.name === e.target.value);
                  if (t) setPreview(renderTemplate(t, selected));
                }}
              >
                {templates
                  .filter(
                    (t) =>
                      !t.whatsapp_account_id ||
                      t.whatsapp_account_id === accountID,
                  )
                  .map((t) => (
                    <option key={t.id} value={t.name}>
                      {t.name.replaceAll("_", " ")}
                    </option>
                  ))}
              </select>
            </label>
            <label>
              Prévia da mensagem
              <textarea className="preview" value={preview} readOnly />
            </label>
            <div className="modal-foot">
              <span>
                <ShieldCheck /> Variáveis preenchidas automaticamente
              </span>
              <button
                className="primary"
                onClick={submit}
                disabled={!accountID || !choice}
              >
                <Send /> Enviar template
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}

function DashboardView({ onReports }: { onReports: () => void }) {
  const [data, setData] = useState<Dashboard | null>(null);
  useEffect(() => {
    api<Dashboard>("/dashboard").then(setData);
  }, []);
  const s = data?.summary || {};
  const cards = [
    { l: "Conversas iniciadas", v: s.conversations_started || 0, c: "lime" },
    { l: "Clientes responderam", v: s.responses || 0, c: "blue" },
    { l: "Atendimentos abertos", v: s.open_conversations || 0, c: "orange" },
    { l: "Vendas realizadas", v: s.sales || 0, c: "purple" },
  ];
  return (
    <>
      <div className="welcome">
        <div>
          <span className="eyebrow green">TERÇA-FEIRA, 11 DE AGOSTO</span>
          <h1>
            Bom dia! <span>👋</span>
          </h1>
          <p>Acompanhe o ritmo do seu time hoje.</p>
        </div>
        <div className="goal">
          <div>
            <span>Meta mensal</span>
            <b>68%</b>
          </div>
          <div className="progress">
            <i />
          </div>
          <small>34 de 50 vendas</small>
        </div>
      </div>
      <div className="metrics">
        {cards.map((x) => (
          <article key={x.l} className={x.c}>
            <div>
              <span>{x.l}</span>
              <b>{x.v}</b>
            </div>
            <span className="trend">
              ↗ 12% <small>vs. ontem</small>
            </span>
          </article>
        ))}
      </div>
      <section className="leaderboard">
        <div className="section-title">
          <div>
            <h3>Desempenho da equipe</h3>
            <p>Resultados de hoje por vendedor</p>
          </div>
          <button className="outline" onClick={onReports}>
            Ver relatório completo
          </button>
        </div>
        {data?.agents.map((a, i) => (
          <div className="agent-row" key={a.id}>
            <span className="rank">{i + 1}</span>
            <span className="avatar">{initials(a.name)}</span>
            <strong>{a.name}</strong>
            <span>
              <b>{a.contacts}</b> contatos
            </span>
            <span>
              <b>{a.sales}</b> vendas
            </span>
            <div className="agent-progress">
              <i
                style={{ width: `${Math.min(100, Number(a.contacts) * 5)}%` }}
              />
            </div>
          </div>
        ))}
      </section>
    </>
  );
}

function ReportsView() {
  const [days, setDays] = useState(30);
  const [data, setData] = useState<ReportData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    setLoading(true);
    setError("");
    api<ReportData>(`/reports?days=${days}`)
      .then(setData)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, [days]);

  const summary = data?.summary || {};
  const cards = [
    ["Conversas iniciadas", summary.conversations_started || 0, "No período"],
    [
      "Atendimentos encerrados",
      summary.conversations_closed || 0,
      "No período",
    ],
    ["Vendas realizadas", summary.sales || 0, "Resultado venda"],
    [
      "Taxa de conversão",
      `${Number(summary.conversion_rate || 0)}%`,
      "Sobre encerrados",
    ],
    [
      "Tempo médio",
      `${Number(summary.average_service_minutes || 0)} min`,
      "Início ao encerramento",
    ],
    ["Em atendimento", summary.open_conversations || 0, "Neste momento"],
  ];
  const dailyMax = Math.max(
    1,
    ...(data?.daily.flatMap((item) => [
      Number(item.started),
      Number(item.closed),
      Number(item.sales),
    ]) || [1]),
  );
  const resultLabels: Record<string, string> = {
    SALE: "Venda",
    INTERESTED: "Cliente interessado",
    CALLBACK: "Retornar depois",
    NO_INTEREST: "Sem interesse",
    NO_RESPONSE: "Sem resposta",
    INVALID_NUMBER: "Número inválido",
    OTHER: "Outro",
  };

  const exportCSV = () => {
    if (!data) return;
    const rows = [
      [
        "Atendente",
        "Contatos",
        "Encerrados",
        "Vendas",
        "Conversão",
        "Em andamento",
      ],
      ...data.agents.map((agent) => [
        agent.name,
        agent.contacts,
        agent.closed,
        agent.sales,
        `${agent.conversion_rate}%`,
        agent.open_now,
      ]),
    ];
    const csv = rows
      .map((row) =>
        row
          .map((value) => `"${String(value).replaceAll('"', '""')}"`)
          .join(";"),
      )
      .join("\n");
    const link = document.createElement("a");
    link.href = URL.createObjectURL(
      new Blob(["\ufeff" + csv], { type: "text/csv;charset=utf-8" }),
    );
    link.download = `zul-desk-relatorio-${days}-dias.csv`;
    link.click();
    URL.revokeObjectURL(link.href);
  };

  return (
    <div className="reports-page">
      <section className="reports-hero">
        <div>
          <span className="eyebrow green">DESEMPENHO OPERACIONAL</span>
          <h1>Relatórios da equipe</h1>
          <p>Compare volume, conversão e produtividade dos atendimentos.</p>
        </div>
        <div className="reports-actions">
          <div className="period-switch" aria-label="Período do relatório">
            {[7, 30, 90].map((value) => (
              <button
                key={value}
                className={days === value ? "active" : ""}
                onClick={() => setDays(value)}
              >
                {value} dias
              </button>
            ))}
          </div>
          <button className="outline" onClick={exportCSV} disabled={!data}>
            <FileText size={15} /> Exportar CSV
          </button>
        </div>
      </section>

      {error && <div className="error">{error}</div>}
      {loading && (
        <div className="report-loading">
          <RefreshCw /> Gerando relatório...
        </div>
      )}
      {!loading && data && (
        <>
          <div className="report-metrics">
            {cards.map(([label, value, hint]) => (
              <article key={String(label)}>
                <span>{label}</span>
                <b>{value}</b>
                <small>{hint}</small>
              </article>
            ))}
          </div>

          <div className="report-grid">
            <section className="report-card daily-card">
              <div className="report-card-title">
                <div>
                  <h3>Evolução diária</h3>
                  <p>Iniciadas, encerradas e vendas por dia</p>
                </div>
                <div className="chart-legend">
                  <span>
                    <i className="started" /> Iniciadas
                  </span>
                  <span>
                    <i className="closed" /> Encerradas
                  </span>
                  <span>
                    <i className="sales" /> Vendas
                  </span>
                </div>
              </div>
              <div className="daily-chart-scroll">
                <div
                  className="daily-chart"
                  style={{ minWidth: Math.max(620, data.daily.length * 22) }}
                >
                  {data.daily.map((item) => (
                    <div
                      className="daily-column"
                      key={item.day}
                      title={`${item.day}: ${item.started} iniciadas, ${item.closed} encerradas, ${item.sales} vendas`}
                    >
                      <div className="daily-bars">
                        <i
                          className="started"
                          style={{
                            height: `${(Number(item.started) / dailyMax) * 100}%`,
                          }}
                        />
                        <i
                          className="closed"
                          style={{
                            height: `${(Number(item.closed) / dailyMax) * 100}%`,
                          }}
                        />
                        <i
                          className="sales"
                          style={{
                            height: `${(Number(item.sales) / dailyMax) * 100}%`,
                          }}
                        />
                      </div>
                      <span>
                        {new Date(item.day).toLocaleDateString("pt-BR", {
                          day: "2-digit",
                          month: "2-digit",
                          timeZone: "UTC",
                        })}
                      </span>
                    </div>
                  ))}
                </div>
              </div>
            </section>

            <section className="report-card results-card">
              <div className="report-card-title">
                <div>
                  <h3>Resultados</h3>
                  <p>Motivos de encerramento</p>
                </div>
              </div>
              <div className="result-list">
                {data.results.length === 0 && (
                  <p className="empty-report">Nenhum atendimento encerrado.</p>
                )}
                {data.results.map((item) => {
                  const total = Math.max(
                    1,
                    Number(summary.conversations_closed || 0),
                  );
                  const percent = Math.round(
                    (Number(item.total) / total) * 100,
                  );
                  return (
                    <div key={item.result}>
                      <span>{resultLabels[item.result] || item.result}</span>
                      <b>{item.total}</b>
                      <div>
                        <i style={{ width: `${percent}%` }} />
                      </div>
                      <small>{percent}%</small>
                    </div>
                  );
                })}
              </div>
              <div className="message-volume">
                <span>
                  <b>{summary.inbound_messages || 0}</b> recebidas
                </span>
                <span>
                  <b>{summary.outbound_messages || 0}</b> enviadas
                </span>
              </div>
            </section>
          </div>

          <section className="report-card team-report">
            <div className="report-card-title">
              <div>
                <h3>Desempenho por atendente</h3>
                <p>Indicadores individuais no período selecionado</p>
              </div>
            </div>
            <div className="report-table">
              <div className="report-table-head">
                <span>Atendente</span>
                <span>Contatos</span>
                <span>Encerrados</span>
                <span>Vendas</span>
                <span>Conversão</span>
                <span>Em andamento</span>
              </div>
              {data.agents.map((agent) => (
                <div className="report-table-row" key={agent.id}>
                  <div>
                    <span className="avatar">{initials(agent.name)}</span>
                    <b>{agent.name}</b>
                  </div>
                  <span>{agent.contacts}</span>
                  <span>{agent.closed}</span>
                  <span className="sales-value">{agent.sales}</span>
                  <span>{agent.conversion_rate}%</span>
                  <span>{agent.open_now}</span>
                </div>
              ))}
            </div>
          </section>
        </>
      )}
    </div>
  );
}

function SettingsView({
  user,
  branding,
  onBranding,
}: {
  user: User;
  branding: Branding;
  onBranding: (b: Branding) => void;
}) {
  const sections = [
    {
      id: "templates",
      label: "Templates",
      description: "Mensagens aprovadas pela Meta",
      icon: FileText,
    },
    ...(user.Role === "ADMIN"
      ? [
          {
            id: "team",
            label: "Equipe",
            description: "Usuários, grupos e permissões",
            icon: UserCog,
          },
          {
            id: "whatsapp",
            label: "Números WhatsApp",
            description: "Contas e credenciais da Cloud API",
            icon: Smartphone,
          },
          {
            id: "branding",
            label: "Marca e aparência",
            description: "Nome, logo e favicon do white label",
            icon: Palette,
          },
        ]
      : []),
  ];
  const [section, setSection] = useState(sections[0].id);
  const current = sections.find((item) => item.id === section) || sections[0];
  return (
    <div className="settings-page">
      <div className="settings-heading">
        <span className="settings-icon">
          <Settings />
        </span>
        <div>
          <h2>Configurações do Zul Desk</h2>
          <p>
            Centralize a operação, as integrações e a identidade da empresa.
          </p>
        </div>
      </div>
      <div className="settings-layout">
        <aside className="settings-menu">
          <small>CONFIGURAÇÕES</small>
          {sections.map((item) => (
            <button
              key={item.id}
              className={section === item.id ? "active" : ""}
              onClick={() => setSection(item.id)}
            >
              <span>
                <item.icon />
              </span>
              <div>
                <b>{item.label}</b>
                <small>{item.description}</small>
              </div>
              <ChevronDown />
            </button>
          ))}
        </aside>
        <section className="settings-content">
          <div className="settings-section-title">
            <div>
              <span className="eyebrow green">CONFIGURAÇÕES</span>
              <h3>{current.label}</h3>
              <p>{current.description}</p>
            </div>
          </div>
          <div className="settings-section-body">
            {section === "templates" && <TemplatesView embedded />}
            {section === "team" && <TeamView currentUser={user} />}
            {section === "whatsapp" && <WhatsAppAccountsView />}
            {section === "branding" && (
              <BrandingSettings branding={branding} onChange={onBranding} />
            )}
          </div>
        </section>
      </div>
    </div>
  );
}

function TemplatesView({ embedded = false }: { embedded?: boolean }) {
  const [items, setItems] = useState<Template[]>([]);
  useEffect(() => {
    api<{ items: Template[] }>("/templates").then((x) => setItems(x.items));
  }, []);
  return (
    <section className={`templates ${embedded ? "embedded" : ""}`}>
      <div className="section-title">
        <div>
          <h3>Templates comerciais</h3>
          <p>Sincronizados com a sua conta do WhatsApp Business</p>
        </div>
        <button className="primary">
          <Wifi size={17} /> Sincronizar Meta
        </button>
      </div>
      {items.map((t) => (
        <article key={t.id}>
          <div className="template-icon">
            <FileText />
          </div>
          <div>
            <h4>{t.name.replaceAll("_", " ")}</h4>
            <p>{t.content}</p>
            <span>
              <Tag /> {t.category}
            </span>
          </div>
          <span
            className={`status ${t.status === "APPROVED" ? "approved" : ""}`}
          >
            {t.status === "APPROVED" ? <Check /> : <Clock3 />}
            {t.status === "APPROVED" ? "Aprovado" : "Em análise"}
          </span>
        </article>
      ))}
    </section>
  );
}
function WhatsAppAccountsView() {
  const [items, setItems] = useState<WhatsAppAccount[]>([]),
    [modal, setModal] = useState(false),
    [tokenAccount, setTokenAccount] = useState<WhatsAppAccount | null>(null),
    [editingAccount, setEditingAccount] = useState<WhatsAppAccount | null>(
      null,
    ),
    [tokenValue, setTokenValue] = useState(""),
    [meta, setMeta] = useState<MetaSettings | null>(null),
    [verifyToken, setVerifyToken] = useState(""),
    [appSecret, setAppSecret] = useState(""),
    [busy, setBusy] = useState(""),
    [message, setMessage] = useState(""),
    [error, setError] = useState("");
  const [form, setForm] = useState({
    name: "",
    business_account_id: "",
    phone_number_id: "",
    display_phone_number: "",
    access_token: "",
    api_version: "v23.0",
    coexistence: false,
  });
  const [editForm, setEditForm] = useState({
    name: "",
    business_account_id: "",
    phone_number_id: "",
    display_phone_number: "",
    api_version: "v23.0",
    coexistence: false,
    active: true,
  });
  const load = useCallback(async () => {
    try {
      const [accounts, settings] = await Promise.all([
        api<{ items: WhatsAppAccount[] }>("/whatsapp/accounts"),
        api<MetaSettings>("/settings/whatsapp"),
      ]);
      setItems(accounts.items);
      setMeta(settings);
      setVerifyToken(settings.verify_token);
    } catch (e) {
      setError((e as Error).message);
    }
  }, []);
  useEffect(() => {
    void load();
  }, [load]);
  async function create(e: FormEvent) {
    e.preventDefault();
    setBusy("create");
    setError("");
    try {
      await api("/whatsapp/accounts", {
        method: "POST",
        body: JSON.stringify(form),
      });
      setModal(false);
      setForm({
        name: "",
        business_account_id: "",
        phone_number_id: "",
        display_phone_number: "",
        access_token: "",
        api_version: "v23.0",
        coexistence: false,
      });
      await load();
      setMessage(
        "Número cadastrado. Use “Testar conexão” para validar na Meta.",
      );
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy("");
    }
  }
  async function action(
    id: string,
    kind: "test" | "sync-phones" | "sync-templates",
  ) {
    setBusy(id + kind);
    setError("");
    setMessage("");
    try {
      const result = await api<Record<string, unknown>>(
        `/whatsapp/accounts/${id}/${kind}`,
        { method: "POST", body: "{}" },
      );
      await load();
      setMessage(
        kind === "test"
          ? "Conexão validada com a Meta."
          : `${String(result.synced || 0)} registros sincronizados.`,
      );
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy("");
    }
  }
  async function updateToken(e: FormEvent) {
    e.preventDefault();
    if (!tokenAccount || !tokenValue.trim()) return;
    setBusy("token");
    setError("");
    try {
      await api(`/whatsapp/accounts/${tokenAccount.id}`, {
        method: "PATCH",
        body: JSON.stringify({ access_token: tokenValue.trim() }),
      });
      setTokenAccount(null);
      setTokenValue("");
      await load();
      setMessage("Token atualizado. Teste novamente a conexão com a Meta.");
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy("");
    }
  }
  async function saveWebhook(e: FormEvent) {
    e.preventDefault();
    setBusy("webhook");
    setError("");
    try {
      const settings = await api<MetaSettings>("/settings/whatsapp", {
        method: "PATCH",
        body: JSON.stringify({
          verify_token: verifyToken.trim(),
          app_secret: appSecret.trim(),
        }),
      });
      setMeta(settings);
      setVerifyToken(settings.verify_token);
      setAppSecret("");
      setMessage("Configuração do webhook salva com segurança.");
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy("");
    }
  }
  async function regenerateVerifyToken() {
    setBusy("regenerate");
    setError("");
    try {
      const settings = await api<MetaSettings>("/settings/whatsapp", {
        method: "PATCH",
        body: JSON.stringify({ regenerate_verify_token: true }),
      });
      setMeta(settings);
      setVerifyToken(settings.verify_token);
      setMessage("Novo token gerado. Atualize-o também no painel da Meta.");
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy("");
    }
  }
  function openEdit(account: WhatsAppAccount) {
    setEditingAccount(account);
    setEditForm({
      name: account.name,
      business_account_id: account.business_account_id,
      phone_number_id: account.phone_number_id,
      display_phone_number: account.display_phone_number || "",
      api_version: account.api_version || "v23.0",
      coexistence: account.coexistence,
      active: account.active,
    });
  }
  async function saveEdit(e: FormEvent) {
    e.preventDefault();
    if (!editingAccount) return;
    setBusy("edit");
    setError("");
    try {
      await api(`/whatsapp/accounts/${editingAccount.id}`, {
        method: "PATCH",
        body: JSON.stringify(editForm),
      });
      setEditingAccount(null);
      await load();
      setMessage("Configurações do número atualizadas.");
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy("");
    }
  }
  return (
    <>
      <div className="accounts-intro">
        <div>
          <span className="eyebrow green">CLOUD API OFICIAL</span>
          <h2>Seus números do WhatsApp</h2>
          <p>
            Cadastre números independentes ou em modo de coexistência com o
            WhatsApp Business do celular.
          </p>
        </div>
        <button className="primary" onClick={() => setModal(true)}>
          <Plus /> Adicionar número
        </button>
      </div>
      {error && <div className="error banner">{error}</div>}
      {message && (
        <div className="success banner">
          <Check />
          {message}
        </div>
      )}
      <form className="webhook-settings" onSubmit={saveWebhook}>
        <div className="webhook-heading">
          <span className="phone-icon">
            <Wifi />
          </span>
          <div>
            <h3>Webhook da Meta</h3>
            <p>Configure por aqui os dados usados para receber mensagens.</p>
          </div>
          <span
            className={`status ${meta?.app_secret_configured ? "approved" : ""}`}
          >
            {meta?.app_secret_configured ? "Protegido" : "App Secret pendente"}
          </span>
        </div>
        <div className="webhook-fields">
          <label className="wide">
            URL de callback
            <div className="copy-field">
              <input readOnly value={meta?.callback_url || "Carregando…"} />
              <button
                type="button"
                className="outline"
                onClick={() =>
                  navigator.clipboard.writeText(meta?.callback_url || "")
                }
              >
                Copiar
              </button>
            </div>
          </label>
          <label>
            Token de verificação
            <input
              required
              value={verifyToken}
              onChange={(e) => setVerifyToken(e.target.value)}
            />
          </label>
          <label>
            App Secret
            <input
              type="password"
              autoComplete="off"
              placeholder={
                meta?.app_secret_configured
                  ? "Já configurado — preencha somente para trocar"
                  : "Cole o App Secret da Meta"
              }
              value={appSecret}
              onChange={(e) => setAppSecret(e.target.value)}
            />
          </label>
        </div>
        <div className="webhook-actions">
          <span>
            <ShieldCheck /> O App Secret nunca volta para o navegador.
          </span>
          <button
            type="button"
            className="outline"
            disabled={busy !== ""}
            onClick={regenerateVerifyToken}
          >
            <RefreshCw /> Gerar token
          </button>
          <button className="primary" disabled={busy !== ""}>
            <Check /> {busy === "webhook" ? "Salvando…" : "Salvar webhook"}
          </button>
        </div>
      </form>
      <div className="account-grid">
        {items.map((a) => (
          <article className="account-card" key={a.id}>
            <div className="account-card-head">
              <span className="phone-icon">
                <Phone />
              </span>
              <div>
                <h3>{a.name}</h3>
                <p>{a.display_phone_number || "Número ainda não consultado"}</p>
              </div>
              <span className={`status ${a.active ? "approved" : ""}`}>
                {a.active ? "Ativo" : "Inativo"}
              </span>
            </div>
            <div className="account-badges">
              <span>
                {a.coexistence ? "Celular + API" : "Somente Cloud API"}
              </span>
              <span>{a.platform_status || "Não testado"}</span>
              {a.quality_rating && <span>Qualidade {a.quality_rating}</span>}
            </div>
            <dl>
              <div>
                <dt>Phone Number ID</dt>
                <dd>{a.phone_number_id}</dd>
              </div>
              <div>
                <dt>WABA ID</dt>
                <dd>{a.business_account_id}</dd>
              </div>
              <div>
                <dt>Nome verificado</dt>
                <dd>{a.verified_name || "—"}</dd>
              </div>
              <div>
                <dt>Token</dt>
                <dd>{a.has_token ? "Protegido e salvo" : "Não configurado"}</dd>
              </div>
            </dl>
            <div className="account-actions">
              <button
                className="outline"
                onClick={() => action(a.id, "test")}
                disabled={busy !== ""}
              >
                <Wifi />{" "}
                {busy === a.id + "test" ? "Testando…" : "Testar conexão"}
              </button>
              <button
                className="outline"
                onClick={() => action(a.id, "sync-phones")}
                disabled={busy !== ""}
              >
                <RefreshCw /> Sincronizar números
              </button>
              <button
                className="outline"
                onClick={() => action(a.id, "sync-templates")}
                disabled={busy !== ""}
              >
                <FileText /> Templates
              </button>
              <button
                className="outline"
                onClick={() => {
                  setTokenAccount(a);
                  setTokenValue("");
                }}
                disabled={busy !== ""}
              >
                <LockKeyhole /> Atualizar token
              </button>
              <button
                className="outline"
                onClick={() => openEdit(a)}
                disabled={busy !== ""}
              >
                <Settings /> Editar
              </button>
            </div>
          </article>
        ))}
      </div>
      {modal && (
        <div className="modal-backdrop">
          <form className="modal account-modal" onSubmit={create}>
            <div className="modal-head">
              <div>
                <span className="eyebrow green">NOVO NÚMERO</span>
                <h3>Conectar à Meta Cloud API</h3>
              </div>
              <button
                type="button"
                className="icon-btn"
                onClick={() => setModal(false)}
              >
                <X />
              </button>
            </div>
            <div className="notice coexistence">
              <Smartphone />
              <div>
                <b>Manter no WhatsApp Business do celular</b>
                <span>
                  Marque coexistência somente após concluir o Embedded Signup
                  específico da Meta.
                </span>
              </div>
            </div>
            <div className="form-grid">
              <label>
                Nome interno
                <input
                  required
                  placeholder="Ex.: Comercial São Gabriel"
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                />
              </label>
              <label>
                Número exibido
                <input
                  placeholder="+55 55 99999-9999"
                  value={form.display_phone_number}
                  onChange={(e) =>
                    setForm({ ...form, display_phone_number: e.target.value })
                  }
                />
              </label>
              <label>
                WhatsApp Business Account ID
                <input
                  required
                  placeholder="WABA ID"
                  value={form.business_account_id}
                  onChange={(e) =>
                    setForm({ ...form, business_account_id: e.target.value })
                  }
                />
              </label>
              <label>
                Phone Number ID
                <input
                  required
                  placeholder="Phone Number ID"
                  value={form.phone_number_id}
                  onChange={(e) =>
                    setForm({ ...form, phone_number_id: e.target.value })
                  }
                />
              </label>
              <label className="wide">
                Access Token
                <input
                  required
                  type="password"
                  autoComplete="off"
                  placeholder="Token temporário ou permanente da Meta"
                  value={form.access_token}
                  onChange={(e) =>
                    setForm({ ...form, access_token: e.target.value })
                  }
                />
                <small>
                  Tokens temporários servem para teste. O token será
                  criptografado e nunca retornará à interface.
                </small>
              </label>
              <label className="check-label wide">
                <input
                  type="checkbox"
                  checked={form.coexistence}
                  onChange={(e) =>
                    setForm({ ...form, coexistence: e.target.checked })
                  }
                />
                <span>
                  <b>Usar coexistência</b>
                  <small>
                    Este número continuará ativo no aplicativo WhatsApp
                    Business.
                  </small>
                </span>
              </label>
            </div>
            <div className="modal-foot">
              <span>
                <ShieldCheck /> Credencial protegida com AES-256-GCM
              </span>
              <button className="primary" disabled={busy === "create"}>
                <Plus />
                {busy === "create" ? "Salvando…" : "Cadastrar número"}
              </button>
            </div>
          </form>
        </div>
      )}
      {tokenAccount && (
        <div className="modal-backdrop">
          <form className="modal action-modal" onSubmit={updateToken}>
            <div className="modal-head">
              <div>
                <span className="eyebrow green">CREDENCIAL DA META</span>
                <h3>Atualizar token</h3>
                <p>{tokenAccount.name}</p>
              </div>
              <button
                type="button"
                className="icon-btn"
                onClick={() => setTokenAccount(null)}
              >
                <X />
              </button>
            </div>
            <div className="notice coexistence">
              <ShieldCheck />
              <div>
                <b>O token é protegido no servidor</b>
                <span>
                  Use esta opção quando o token temporário da Meta expirar.
                </span>
              </div>
            </div>
            <label>
              Novo Access Token
              <input
                required
                type="password"
                autoComplete="off"
                placeholder="Cole o novo token da Meta"
                value={tokenValue}
                onChange={(e) => setTokenValue(e.target.value)}
              />
            </label>
            <div className="modal-foot">
              <span>
                <LockKeyhole /> A credencial atual não será exibida
              </span>
              <button className="primary" disabled={busy === "token"}>
                <Check />
                {busy === "token" ? "Atualizando…" : "Salvar novo token"}
              </button>
            </div>
          </form>
        </div>
      )}
      {editingAccount && (
        <div className="modal-backdrop">
          <form className="modal account-modal" onSubmit={saveEdit}>
            <div className="modal-head">
              <div>
                <span className="eyebrow green">CONFIGURAR NÚMERO</span>
                <h3>{editingAccount.name}</h3>
              </div>
              <button
                type="button"
                className="icon-btn"
                onClick={() => setEditingAccount(null)}
              >
                <X />
              </button>
            </div>
            <div className="form-grid">
              <label>
                Nome interno
                <input
                  required
                  value={editForm.name}
                  onChange={(e) =>
                    setEditForm({ ...editForm, name: e.target.value })
                  }
                />
              </label>
              <label>
                Número exibido
                <input
                  value={editForm.display_phone_number}
                  onChange={(e) =>
                    setEditForm({
                      ...editForm,
                      display_phone_number: e.target.value,
                    })
                  }
                />
              </label>
              <label>
                WhatsApp Business Account ID
                <input
                  required
                  value={editForm.business_account_id}
                  onChange={(e) =>
                    setEditForm({
                      ...editForm,
                      business_account_id: e.target.value,
                    })
                  }
                />
              </label>
              <label>
                Phone Number ID
                <input
                  required
                  value={editForm.phone_number_id}
                  onChange={(e) =>
                    setEditForm({
                      ...editForm,
                      phone_number_id: e.target.value,
                    })
                  }
                />
              </label>
              <label>
                Versão da Graph API
                <input
                  required
                  value={editForm.api_version}
                  onChange={(e) =>
                    setEditForm({ ...editForm, api_version: e.target.value })
                  }
                />
              </label>
              <label className="check-label">
                <input
                  type="checkbox"
                  checked={editForm.active}
                  onChange={(e) =>
                    setEditForm({ ...editForm, active: e.target.checked })
                  }
                />
                <span>
                  <b>Número ativo</b>
                  <small>Disponível para novos atendimentos.</small>
                </span>
              </label>
              <label className="check-label wide">
                <input
                  type="checkbox"
                  checked={editForm.coexistence}
                  onChange={(e) =>
                    setEditForm({ ...editForm, coexistence: e.target.checked })
                  }
                />
                <span>
                  <b>Usar coexistência</b>
                  <small>
                    Mantenha marcado somente após o onboarding de coexistência
                    da Meta.
                  </small>
                </span>
              </label>
            </div>
            <div className="modal-foot">
              <span>
                <ShieldCheck /> Identificadores e status gerenciados pela
                interface
              </span>
              <button className="primary" disabled={busy === "edit"}>
                <Check /> {busy === "edit" ? "Salvando…" : "Salvar alterações"}
              </button>
            </div>
          </form>
        </div>
      )}
    </>
  );
}
const roleLabel = (role: string) =>
  role === "ADMIN"
    ? "Administrador"
    : role === "SUPERVISOR"
      ? "Supervisor"
      : "Usuário";
function TeamView({ currentUser }: { currentUser: User }) {
  const [items, setItems] = useState<TeamUser[]>([]),
    [groups, setGroups] = useState<UserGroup[]>([]),
    [tab, setTab] = useState<"people" | "groups">("people"),
    [modal, setModal] = useState<"user" | "group" | null>(null),
    [editing, setEditing] = useState<TeamUser | null>(null),
    [editingGroup, setEditingGroup] = useState<UserGroup | null>(null),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [message, setMessage] = useState("");
  const [userForm, setUserForm] = useState({
      name: "",
      email: "",
      password: "",
      role: "AGENT",
      group_ids: [] as string[],
    }),
    [groupForm, setGroupForm] = useState({
      name: "",
      description: "",
      color: "#15a76e",
      user_ids: [] as string[],
    });
  const load = useCallback(async () => {
    try {
      const [u, g] = await Promise.all([
        api<{ items: TeamUser[] }>("/users"),
        api<{ items: UserGroup[] }>("/groups"),
      ]);
      setItems(u.items);
      setGroups(g.items);
    } catch (e) {
      setError((e as Error).message);
    }
  }, []);
  useEffect(() => {
    void load();
  }, [load]);
  function openUser(item?: TeamUser) {
    setEditing(item || null);
    setUserForm(
      item
        ? {
            name: item.name,
            email: item.email,
            password: "",
            role: item.role,
            group_ids: item.group_ids || [],
          }
        : { name: "", email: "", password: "", role: "AGENT", group_ids: [] },
    );
    setError("");
    setModal("user");
  }
  function openGroup(group?: UserGroup) {
    setEditingGroup(group || null);
    setGroupForm(
      group
        ? {
            name: group.name,
            description: group.description,
            color: group.color,
            user_ids: group.user_ids || [],
          }
        : { name: "", description: "", color: "#15a76e", user_ids: [] },
    );
    setError("");
    setModal("group");
  }
  const toggleID = (values: string[], id: string) =>
    values.includes(id) ? values.filter((x) => x !== id) : [...values, id];
  async function saveUser(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    setMessage("");
    try {
      if (editing)
        await api(`/users/${editing.id}`, {
          method: "PATCH",
          body: JSON.stringify({
            name: userForm.name,
            role: userForm.role,
            password: userForm.password || undefined,
            group_ids: userForm.group_ids,
          }),
        });
      else
        await api("/users", { method: "POST", body: JSON.stringify(userForm) });
      setModal(null);
      setMessage(
        editing ? "Acesso e grupos atualizados." : "Novo usuário cadastrado.",
      );
      await load();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function saveGroup(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    setMessage("");
    try {
      if (editingGroup)
        await api(`/groups/${editingGroup.id}`, {
          method: "PATCH",
          body: JSON.stringify(groupForm),
        });
      else
        await api("/groups", {
          method: "POST",
          body: JSON.stringify(groupForm),
        });
      setModal(null);
      setMessage(editingGroup ? "Grupo atualizado." : "Novo grupo criado.");
      await load();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function toggle(item: TeamUser) {
    setError("");
    try {
      await api(`/users/${item.id}`, {
        method: "PATCH",
        body: JSON.stringify({ active: !item.active }),
      });
      setMessage(item.active ? "Acesso desativado." : "Acesso reativado.");
      await load();
    } catch (e) {
      setError((e as Error).message);
    }
  }
  async function toggleGroup(group: UserGroup) {
    setError("");
    try {
      await api(`/groups/${group.id}`, {
        method: "PATCH",
        body: JSON.stringify({ active: !group.active }),
      });
      setMessage(group.active ? "Grupo arquivado." : "Grupo reativado.");
      await load();
    } catch (e) {
      setError((e as Error).message);
    }
  }
  return (
    <>
      <div className="accounts-intro team-intro">
        <div>
          <span className="eyebrow green">PESSOAS E GRUPOS</span>
          <h2>Organize sua operação</h2>
          <p>
            Separe permissões de acesso das áreas como Vendas, Financeiro e
            Suporte.
          </p>
        </div>
        <button
          className="primary"
          onClick={() => (tab === "people" ? openUser() : openGroup())}
        >
          <Plus /> {tab === "people" ? "Adicionar usuário" : "Criar grupo"}
        </button>
      </div>
      <div className="management-tabs">
        <button
          className={tab === "people" ? "active" : ""}
          onClick={() => setTab("people")}
        >
          <Users /> Usuários <b>{items.length}</b>
        </button>
        <button
          className={tab === "groups" ? "active" : ""}
          onClick={() => setTab("groups")}
        >
          <UserRound /> Grupos <b>{groups.filter((g) => g.active).length}</b>
        </button>
      </div>
      {error && !modal && <div className="error banner">{error}</div>}
      {message && (
        <div className="success banner">
          <Check />
          {message}
        </div>
      )}
      {tab === "people" ? (
        <section className="team-list">
          <div className="team-head">
            <span>PESSOA</span>
            <span>PERFIL E GRUPOS</span>
            <span>STATUS</span>
            <span>ACESSO</span>
          </div>
          {items.map((item) => (
            <article
              className={`team-row ${!item.active ? "inactive" : ""}`}
              key={item.id}
            >
              <div className="person">
                <span className="avatar">{initials(item.name)}</span>
                <div>
                  <strong>
                    {item.name}
                    {item.id === currentUser.ID && <small>Você</small>}
                  </strong>
                  <span>{item.email}</span>
                </div>
              </div>
              <div className="user-roles">
                <span className={`role-badge ${item.role.toLowerCase()}`}>
                  {roleLabel(item.role)}
                </span>
                <div className="mini-groups">
                  {groups
                    .filter((g) => item.group_ids?.includes(g.id))
                    .map((g) => (
                      <span
                        key={g.id}
                        style={{ borderColor: g.color, color: g.color }}
                      >
                        {g.name}
                      </span>
                    ))}
                  {!item.group_ids?.length && <small>Sem grupo</small>}
                </div>
              </div>
              <span className={`access-status ${item.active ? "on" : ""}`}>
                <i />
                {item.active ? "Ativo" : "Inativo"}
              </span>
              <div className="team-actions">
                <button className="outline" onClick={() => openUser(item)}>
                  Editar
                </button>
                <button
                  className="outline"
                  disabled={item.id === currentUser.ID}
                  onClick={() => toggle(item)}
                >
                  {item.active ? "Desativar" : "Reativar"}
                </button>
              </div>
            </article>
          ))}
        </section>
      ) : (
        <div className="group-grid">
          {groups.map((group) => (
            <article
              className={`group-card ${!group.active ? "inactive" : ""}`}
              key={group.id}
            >
              <div className="group-card-head">
                <span style={{ background: group.color }}>
                  <UserRound />
                </span>
                <div>
                  <h3>{group.name}</h3>
                  <p>{group.description || "Sem descrição"}</p>
                </div>
                <span className={`access-status ${group.active ? "on" : ""}`}>
                  <i />
                  {group.active ? "Ativo" : "Arquivado"}
                </span>
              </div>
              <div className="member-stack">
                {items
                  .filter((u) => group.user_ids?.includes(u.id))
                  .slice(0, 5)
                  .map((u) => (
                    <span className="avatar" title={u.name} key={u.id}>
                      {initials(u.name)}
                    </span>
                  ))}
                <b>
                  {Number(group.member_count)}{" "}
                  {Number(group.member_count) === 1 ? "membro" : "membros"}
                </b>
              </div>
              <div className="group-actions">
                <button className="outline" onClick={() => openGroup(group)}>
                  Editar membros
                </button>
                <button className="outline" onClick={() => toggleGroup(group)}>
                  {group.active ? "Arquivar" : "Reativar"}
                </button>
              </div>
            </article>
          ))}
        </div>
      )}
      {modal === "user" && (
        <div className="modal-backdrop">
          <form className="modal team-modal" onSubmit={saveUser}>
            <div className="modal-head">
              <div>
                <span className="eyebrow green">
                  {editing ? "EDITAR USUÁRIO" : "NOVO USUÁRIO"}
                </span>
                <h3>{editing ? editing.name : "Adicionar à equipe"}</h3>
              </div>
              <button
                type="button"
                className="icon-btn"
                onClick={() => setModal(null)}
              >
                <X />
              </button>
            </div>
            {error && <div className="error banner">{error}</div>}
            <div className="form-grid">
              <label>
                Nome completo
                <input
                  required
                  value={userForm.name}
                  onChange={(e) =>
                    setUserForm({ ...userForm, name: e.target.value })
                  }
                />
              </label>
              <label>
                E-mail
                <input
                  type="email"
                  required
                  disabled={!!editing}
                  value={userForm.email}
                  onChange={(e) =>
                    setUserForm({ ...userForm, email: e.target.value })
                  }
                />
              </label>
              <label>
                Perfil de permissão
                <select
                  value={userForm.role}
                  onChange={(e) =>
                    setUserForm({ ...userForm, role: e.target.value })
                  }
                >
                  <option value="AGENT">Usuário</option>
                  <option value="SUPERVISOR">Supervisor</option>
                  <option value="ADMIN">Administrador</option>
                </select>
                <small>
                  O perfil controla as permissões, não o departamento.
                </small>
              </label>
              <label>
                {editing ? "Nova senha (opcional)" : "Senha inicial"}
                <input
                  type="password"
                  required={!editing}
                  minLength={8}
                  autoComplete="new-password"
                  value={userForm.password}
                  onChange={(e) =>
                    setUserForm({ ...userForm, password: e.target.value })
                  }
                />
                <small>Mínimo de 8 caracteres.</small>
              </label>
              <fieldset className="group-picker wide">
                <legend>Grupos do usuário</legend>
                {groups
                  .filter((g) => g.active)
                  .map((g) => (
                    <label key={g.id}>
                      <input
                        type="checkbox"
                        checked={userForm.group_ids.includes(g.id)}
                        onChange={() =>
                          setUserForm({
                            ...userForm,
                            group_ids: toggleID(userForm.group_ids, g.id),
                          })
                        }
                      />
                      <i style={{ background: g.color }} />
                      <span>
                        <b>{g.name}</b>
                        <small>{g.description}</small>
                      </span>
                    </label>
                  ))}
              </fieldset>
            </div>
            <div className="modal-foot">
              <span>
                <ShieldCheck /> Um usuário pode participar de vários grupos.
              </span>
              <button className="primary" disabled={busy}>
                <Check />
                {busy
                  ? "Salvando…"
                  : editing
                    ? "Salvar alterações"
                    : "Criar acesso"}
              </button>
            </div>
          </form>
        </div>
      )}
      {modal === "group" && (
        <div className="modal-backdrop">
          <form className="modal team-modal" onSubmit={saveGroup}>
            <div className="modal-head">
              <div>
                <span className="eyebrow green">
                  {editingGroup ? "EDITAR GRUPO" : "NOVO GRUPO"}
                </span>
                <h3>{editingGroup ? editingGroup.name : "Criar grupo"}</h3>
              </div>
              <button
                type="button"
                className="icon-btn"
                onClick={() => setModal(null)}
              >
                <X />
              </button>
            </div>
            {error && <div className="error banner">{error}</div>}
            <div className="form-grid">
              <label>
                Nome do grupo
                <input
                  required
                  maxLength={80}
                  placeholder="Ex.: Financeiro"
                  value={groupForm.name}
                  onChange={(e) =>
                    setGroupForm({ ...groupForm, name: e.target.value })
                  }
                />
              </label>
              <label>
                Cor de identificação
                <div className="color-input">
                  <input
                    type="color"
                    value={groupForm.color}
                    onChange={(e) =>
                      setGroupForm({ ...groupForm, color: e.target.value })
                    }
                  />
                  <span>{groupForm.color}</span>
                </div>
              </label>
              <label className="wide">
                Descrição
                <input
                  placeholder="Responsabilidade deste grupo"
                  value={groupForm.description}
                  onChange={(e) =>
                    setGroupForm({ ...groupForm, description: e.target.value })
                  }
                />
              </label>
              <fieldset className="group-picker wide">
                <legend>Membros do grupo</legend>
                {items
                  .filter((u) => u.active)
                  .map((u) => (
                    <label key={u.id}>
                      <input
                        type="checkbox"
                        checked={groupForm.user_ids.includes(u.id)}
                        onChange={() =>
                          setGroupForm({
                            ...groupForm,
                            user_ids: toggleID(groupForm.user_ids, u.id),
                          })
                        }
                      />
                      <span className="avatar">{initials(u.name)}</span>
                      <span>
                        <b>{u.name}</b>
                        <small>{u.email}</small>
                      </span>
                    </label>
                  ))}
              </fieldset>
            </div>
            <div className="modal-foot">
              <span>
                <ShieldCheck /> Os membros podem participar de outros grupos.
              </span>
              <button className="primary" disabled={busy}>
                <Check />
                {busy
                  ? "Salvando…"
                  : editingGroup
                    ? "Salvar grupo"
                    : "Criar grupo"}
              </button>
            </div>
          </form>
        </div>
      )}
    </>
  );
}
function BrandingSettings({
  branding,
  onChange,
}: {
  branding: Branding;
  onChange: (b: Branding) => void;
}) {
  const [appName, setAppName] = useState(branding.app_name),
    [companyName, setCompanyName] = useState(branding.company_name),
    [logo, setLogo] = useState<File | null>(null),
    [favicon, setFavicon] = useState<File | null>(null),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [saved, setSaved] = useState(false);
  const logoPreview = useMemo(
    () => (logo ? URL.createObjectURL(logo) : branding.logo_url || ""),
    [logo, branding.logo_url],
  );
  const faviconPreview = useMemo(
    () => (favicon ? URL.createObjectURL(favicon) : branding.favicon_url || ""),
    [favicon, branding.favicon_url],
  );
  useEffect(
    () => () => {
      if (logoPreview.startsWith("blob:")) URL.revokeObjectURL(logoPreview);
      if (faviconPreview.startsWith("blob:"))
        URL.revokeObjectURL(faviconPreview);
    },
    [logoPreview, faviconPreview],
  );
  async function save(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    setSaved(false);
    const form = new FormData();
    form.set("app_name", appName);
    form.set("company_name", companyName);
    if (logo) form.set("logo", logo);
    if (favicon) form.set("favicon", favicon);
    try {
      const updated = await apiForm<Branding>("/settings/branding", form);
      onChange(updated);
      setLogo(null);
      setFavicon(null);
      setSaved(true);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <form className="branding-settings" onSubmit={save}>
      <div className="branding-editor">
        <div className="section-title">
          <div>
            <h3>Identidade do white label</h3>
            <p>As alterações aparecem para todos os usuários imediatamente.</p>
          </div>
          <button className="primary" disabled={busy}>
            <Check />
            {busy ? "Salvando…" : "Salvar alterações"}
          </button>
        </div>
        {error && <div className="error banner">{error}</div>}
        {saved && (
          <div className="success banner">
            <Check />
            Identidade visual atualizada.
          </div>
        )}
        <div className="branding-fields">
          <label>
            Nome do aplicativo
            <input
              maxLength={60}
              required
              value={appName}
              onChange={(e) => setAppName(e.target.value)}
              placeholder="Zul Desk"
            />
            <small>Aparece no login, menu e título da aba.</small>
          </label>
          <label>
            Nome da empresa cliente
            <input
              maxLength={100}
              required
              value={companyName}
              onChange={(e) => setCompanyName(e.target.value)}
              placeholder="New Life"
            />
            <small>Identifica o workspace comercial.</small>
          </label>
          <label className="upload-field">
            <span>Logo do aplicativo</span>
            <div className="upload-box">
              {logoPreview ? (
                <img src={logoPreview} alt="Prévia da logo" />
              ) : (
                <ImageIcon />
              )}
              <div>
                <b>{logo ? logo.name : "Escolher logo"}</b>
                <small>PNG, JPG ou WebP · até 5 MB</small>
              </div>
              <input
                type="file"
                accept="image/png,image/jpeg,image/webp"
                onChange={(e) => setLogo(e.target.files?.[0] || null)}
              />
            </div>
          </label>
          <label className="upload-field">
            <span>Favicon</span>
            <div className="upload-box favicon-box">
              {faviconPreview ? (
                <img src={faviconPreview} alt="Prévia do favicon" />
              ) : (
                <ImageIcon />
              )}
              <div>
                <b>{favicon ? favicon.name : "Escolher favicon"}</b>
                <small>PNG, JPG, WebP ou ICO · até 5 MB</small>
              </div>
              <input
                type="file"
                accept="image/png,image/jpeg,image/webp,image/x-icon,.ico"
                onChange={(e) => setFavicon(e.target.files?.[0] || null)}
              />
            </div>
          </label>
        </div>
      </div>
      <aside className="brand-preview">
        <span className="eyebrow green">PRÉVIA</span>
        <div className="preview-window">
          <div className="preview-sidebar">
            <BrandLogo
              branding={{
                ...branding,
                app_name: appName,
                company_name: companyName,
                logo_url: logoPreview || null,
              }}
            />
            <div className="preview-company">
              <span className="avatar company">
                {initials(companyName || "Z")}
              </span>
              <b>{companyName || "Sua empresa"}</b>
            </div>
            <i />
            <i />
            <i />
          </div>
          <div className="preview-main">
            <div>
              {faviconPreview ? (
                <img src={faviconPreview} alt="" />
              ) : (
                <span className="brand-mark">
                  <MessageCircleMore />
                </span>
              )}
              <b>{appName || "Seu aplicativo"}</b>
            </div>
            <span>Atendimento comercial via WhatsApp</span>
          </div>
        </div>
        <div className="security-note">
          <ShieldCheck />
          <div>
            <b>Arquivos protegidos e persistentes</b>
            <span>
              Logo e favicon ficam no volume Docker e não precisam de nova
              compilação.
            </span>
          </div>
        </div>
      </aside>
    </form>
  );
}
function Empty({
  icon,
  title,
  text,
}: {
  icon: React.ReactNode;
  title: string;
  text: string;
}) {
  return (
    <div className="empty">
      <span>{icon}</span>
      <b>{title}</b>
      <p>{text}</p>
    </div>
  );
}
