import React from 'react';
import { indicatorSummaryWraps } from '../../shared/passive-indicator-layout';
import { MorphingIndicatorText } from './morphing-text';

export function IndicatorSummaryText({
  value,
  animate,
}: {
  value: string;
  animate: boolean;
}) {
  if (!animate) {
    return value;
  }
  if (indicatorSummaryWraps(value)) {
    return <span className="indicatorSummaryWrapped">{value}</span>;
  }
  return <MorphingIndicatorText value={value} />;
}
