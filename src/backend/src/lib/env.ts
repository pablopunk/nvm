export function env(name: string): string | undefined {
  const metaEnv = (import.meta as unknown as { env?: Record<string, string | undefined> }).env;
  return metaEnv?.[name] ?? process.env[name];
}
