import './globals.css'
import type { Metadata } from 'next'

export const metadata: Metadata = {
  title: 'Audio Player',
  description: 'Audio Player with direct and streaming modes',
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  )
} 