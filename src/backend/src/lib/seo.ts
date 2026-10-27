export const publicPages: Record<
  string,
  { title: string; description: string }
> = {
  '/': {
    title: 'Nevermind: A Raycast alternative that builds its own tools',
    description:
      'A free Raycast alternative for macOS, Windows, and Linux. Launch apps, find files, search clipboard history, and build reusable commands with AI.',
  },
  '/pricing': {
    title: 'Free and Pro plans · Nevermind Pricing',
    description:
      'Start free with built-in desktop tools and monthly AI credits. Compare Nevermind Free and Pro, or bring your own AI to build personal extensions.',
  },
  '/docs': {
    title: 'Desktop setup and custom extensions · Nevermind Docs',
    description:
      'Install Nevermind, learn the command palette, and build your own extensions. Explore app launching, clipboard history, shortcuts, and the AI builder.',
  },
  '/docs/features': {
    title: 'Built-in features · Nevermind Docs',
    description:
      'Every command and tool that ships with Nevermind, generated from the app itself so it always matches the version you download.',
  },
  '/docs/api': {
    title: 'Extension API reference · Nevermind Docs',
    description:
      'The complete Nevermind extension API: manifests, commands, the extension context, and every UI primitive, generated from the typed declaration file.',
  },
  '/terms': {
    title: 'Terms of Service · Nevermind',
    description:
      'Read the terms for using Nevermind, its desktop app, AI features, and subscription services.',
  },
  '/privacy': {
    title: 'Privacy Policy · Nevermind',
    description:
      'Learn how Nevermind handles account information, desktop app data, AI requests, and your privacy rights.',
  },
  '/refund': {
    title: 'Refund Policy · Nevermind',
    description:
      'Read the Nevermind refund policy, including subscription cancellation and how to request help with a payment.',
  },
};
