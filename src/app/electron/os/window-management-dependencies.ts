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
}

export interface ApplyRequest {
  spec: WindowLayoutSpec;
  options: WindowManagementOptions;
  source: WindowInvocation['source'];
  target: WindowFocusTarget;
}

export interface OperationContext {
  dependencies: WindowManagementServiceDependencies;
  memory: WindowMemory;
  request: ApplyRequest;
}
