export interface BuilderPreview {
  filename: string;
  preview: {
    extensionId: string;
    rootItems: unknown[];
    commands: unknown[];
    actions: unknown[];
  };
  autoRunResult?: unknown;
}
