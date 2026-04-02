import "./globals.css";

export const metadata = {
  title: "AmneziaWG Web UI",
  description: "Web interface for managing AmneziaWG peers"
};

export default function RootLayout({ children }) {
  return (
    <html lang="ru">
      <body>{children}</body>
    </html>
  );
}
