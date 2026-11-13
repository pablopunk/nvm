import React from 'react';

type StatusIconKind = 'success' | 'error' | 'info';

const STATUS_ICON_PATHS: Record<StatusIconKind, string[]> = {
  success: ['M6.5 10.5l2.5 2.5 4.5-5.5'],
  error: ['M7 7l6 6', 'M13 7l-6 6'],
  info: ['M10 9.5v4', 'M10 6.8v.1'],
};

export function indicatorStatusIconKind(normalizedStatus: string) {
  if (normalizedStatus === 'done') {
    return 'success';
  }
  if (normalizedStatus === 'error') {
    return 'error';
  }
  return 'info';
}

export function IndicatorStatusIcon({ kind }: { kind: StatusIconKind }) {
  return (
    <svg
      className="indicatorStatusIcon"
      data-kind={kind}
      viewBox="0 0 20 20"
      width="20"
      height="20"
      fill="none"
      aria-hidden="true"
    >
      <circle className="indicatorStatusRing" cx="10" cy="10" r="8" />
      {STATUS_ICON_PATHS[kind].map((path) => (
        <path key={path} className="indicatorStatusGlyph" d={path} />
      ))}
    </svg>
  );
}
