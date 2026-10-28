export interface Stat {
  dev: number;
  ino: number;
  uid: number;
  isDirectory: () => boolean;
  isSymbolicLink: () => boolean;
}

export type Identity = Pick<Stat, 'dev' | 'ino' | 'uid'> & {
  type: 'directory' | 'file';
};

export interface AppUninstallCandidate {
  id: string;
  path: string;
  kind: 'app' | 'associated';
  slot: string;
  selectedByDefault: boolean;
  reason: string;
}

export type SnapshotCandidate = AppUninstallCandidate & {
  identity: Identity;
  root: string;
};

export interface AppUninstallSnapshot {
  appPath: string;
  appBundleId: string;
  appIdentity: Identity;
  candidates: SnapshotCandidate[];
}

export interface AppUninstallNote {
  code: string;
  message: string;
}

export interface Refusal {
  code: string;
  message: string;
}

export type Inspection = { canonical: string; identity: Identity } | Refusal;

export type DiscoveryResult =
  | { status: 'unavailable'; reasonCode: string; message: string }
  | {
      status: 'ready';
      snapshot: AppUninstallSnapshot;
      candidates: AppUninstallCandidate[];
      notes: AppUninstallNote[];
      appRunning: boolean;
    };

export interface UntouchedItem extends Refusal {
  path: string;
}

export interface TrashResult {
  status: 'complete' | 'partial' | 'failed';
  moved: string[];
  untouched: UntouchedItem[];
  notes: AppUninstallNote[];
}

export interface CandidateSpec {
  slot: string;
  path: string;
  root: string;
  kind: 'app' | 'associated';
}

export interface AppUninstallDependencies {
  platform: string;
  homeDirectory: string;
  currentUid: number;
  lstat: (value: string) => Promise<Stat>;
  realpath: (value: string) => Promise<string>;
  access: (value: string, mode: number) => Promise<void>;
  listDirectory: (value: string) => Promise<string[]>;
  readBundleId: (appPath: string) => Promise<unknown>;
  findMatchingApps?: (bundleId: string) => Promise<string[]>;
  trashItem: (value: string) => Promise<void>;
  requestQuit: (appPath: string) => Promise<void>;
  runningAppPaths: (appPath: string) => Promise<Set<string>>;
  nevermindAppPath?: string | null;
  nevermindBundleId?: string | null;
  randomId?: () => string;
  wait?: (ms: number) => Promise<void>;
  quitTimeoutMs?: number;
  quitPollMs?: number;
}
