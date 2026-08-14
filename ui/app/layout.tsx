import type { Metadata } from 'next'
import { Inter } from 'next/font/google'
import Link from 'next/link'
import './globals.css'
import {
  Server,
  Bot,
  Activity,
  Store,
} from 'lucide-react'

const inter = Inter({ subsets: ['latin'] })

export const metadata: Metadata = {
  title: 'MCP Gateway',
  description: 'Manage MCP servers, agents, and policies on Kubernetes',
}

const navItems = [
  { href: '/servers', label: 'Servers', icon: Server },
  { href: '/agents', label: 'Agents', icon: Bot },
  { href: '/monitoring', label: 'Monitoring', icon: Activity },
  { href: '/marketplace', label: 'Marketplace', icon: Store },
]

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="en">
      <body className={inter.className}>
        <div className="flex h-screen">
          {/* Sidebar */}
          <aside className="w-16 flex-shrink-0 border-r border-border bg-card flex flex-col items-center py-4 gap-1">
            <div className="mb-4 flex h-10 w-10 items-center justify-center rounded-lg bg-primary text-primary-foreground font-bold text-sm">
              MG
            </div>
            {navItems.map((item) => (
              <Link
                key={item.href}
                href={item.href}
                className="flex flex-col items-center justify-center w-12 h-12 rounded-lg text-muted-foreground hover:text-foreground hover:bg-muted transition-colors"
                title={item.label}
              >
                <item.icon className="h-5 w-5" />
                <span className="text-[10px] mt-0.5">{item.label}</span>
              </Link>
            ))}
          </aside>

          {/* Main content */}
          <main className="flex-1 overflow-auto">
            {children}
          </main>
        </div>
      </body>
    </html>
  )
}
