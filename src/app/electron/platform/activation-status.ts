export type ActivationStatus = {
  set: (key: string, message: string | null) => void;
  failureFor: (key: string) => string | null;
};

export function createActivationStatus(): ActivationStatus {
  const failures = new Map<string, string>();
  return {
    set(key, message) {
      if (message) failures.set(key, message);
      else failures.delete(key);
    },
    failureFor(key) {
      return failures.get(key) ?? null;
    },
  };
}
