import type { Metadata } from "next";
import "./globals.css";
export const metadata: Metadata = { title: "Zul Desk", description: "Atendimento comercial pelo WhatsApp" };
export default function RootLayout({children}:{children:React.ReactNode}) { return <html lang="pt-BR"><body>{children}</body></html>; }
