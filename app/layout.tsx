import type { Metadata } from 'next';
import './globals.css';

export const metadata: Metadata = {
  title: 'Sprintly — управление задачами',
  description: 'Командная канбан-доска для планирования и доставки продукта.',
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="ru">
      <body>{children}</body>
    </html>
  );
}
