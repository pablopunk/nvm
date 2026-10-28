import type { WindowManagementEngine } from './window-management-engine-port';
import type { WindowMemory } from './window-management-memory';
import type {
  MacosWindowManager,
  WindowFocusTarget,
  WindowInvocation,
  WindowInvocationPorts,
  WindowLayoutSpec,
  WindowManagementOptions,
} from './window-management-types';

export interface WindowManagementServiceDependencies {
  manager: MacosWindowManager;
  ports: WindowInvocationPorts;
  engine: WindowManagementEngine;
  supported(): boolean;
  selfPid?: number;
  now?: () => number;
  createHandleId?: () => string;
}

export type WindowSelection =
  | { kind: 'focused'; source: WindowInvocation['source'] }
  | { kind: 'handle'; windowKey: string };

export interface ApplyRequest {
  spec: WindowLayoutSpec;
  options: WindowManagementOptions;
  selection: WindowSelection;
  target: WindowFocusTarget;
}

export interface OperationContext {
  dependencies: WindowManagementServiceDependencies;
  memory: WindowMemory;
  request: ApplyRequest;
}
