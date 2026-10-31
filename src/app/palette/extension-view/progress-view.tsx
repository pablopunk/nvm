import type { ReactNode } from 'react';
import { IndicatorMicTrace } from '../indicator/indicator-mic-trace';
import { IndicatorOrb, indicatorOrbState } from '../indicator/indicator-orb';
import {
  IndicatorStatusIcon,
  indicatorStatusIconKind,
} from '../indicator/indicator-status-icon';
import { IndicatorSummaryText } from '../indicator/indicator-summary-text';
import { MorphingIndicatorText } from '../indicator/morphing-text';

export type ProgressStep = { title: string; status?: string };

export interface ProgressViewProps {
  steps: ProgressStep[];
  value?: number;
  total?: number;
  label?: string;
  status?: string;
  animateSummaryText?: boolean;
  actions?: ReactNode;
}

export function normalizedProgressStatus(status?: string) {
  const value = String(status || '').toLowerCase();
  if (['done', 'complete', 'completed', 'success'].includes(value))
    return 'done';
  if (
    ['active', 'running', 'loading', 'in progress', 'progress'].includes(value)
  )
    return 'active';
  if (['error', 'failed', 'failure'].includes(value)) return 'error';
  return 'pending';
}

/** Explicit `value`/`total` win; otherwise steps count completed work, so an active step never reads as finished. */
export function progressAmount({
  steps,
  value,
  total,
}: Pick<ProgressViewProps, 'steps' | 'value' | 'total'>) {
  if (typeof value === 'number' && typeof total === 'number' && total > 0)
    return { value, total };
  if (!steps.length) return null;
  const completed = steps.filter(
    (step) => normalizedProgressStatus(step.status) === 'done',
  ).length;
  return { value: completed, total: steps.length };
}

function progressDetail({
  label,
  status,
  animateSummaryText,
}: Pick<ProgressViewProps, 'label' | 'status' | 'animateSummaryText'>) {
  const statusIsOnlyAState =
    normalizedProgressStatus(status) !== 'pending' ||
    status?.toLowerCase() === 'pending';
  return !animateSummaryText && label && status && !statusIsOnlyAState
    ? status
    : '';
}

function ProgressStepRow({ step }: { step: ProgressStep }) {
  return (
    <div
      className="progressStep"
      data-status={normalizedProgressStatus(step.status)}
    >
      <span className="progressStepMarker" aria-hidden="true" />
      <div>
        <strong>{step.title}</strong>
        <small>{step.status || 'Pending'}</small>
      </div>
    </div>
  );
}

export function ProgressView({
  steps,
  value,
  total,
  label,
  status,
  animateSummaryText = false,
  actions,
}: ProgressViewProps) {
  const normalizedStatus = normalizedProgressStatus(status);
  const amount = progressAmount({ steps, value, total });
  const ratio = amount
    ? Math.max(0, Math.min(1, amount.value / amount.total))
    : 0;
  const percent = Math.round(ratio * 100);
  const showSummary = Boolean(label || status) || Boolean(amount);
  const summaryLabel = label || status || 'Working…';
  const detail = progressDetail({ label, status, animateSummaryText });
  const progressLabel = amount
    ? `${amount.value} of ${amount.total} · ${percent}%`
    : '';
  const orbState = animateSummaryText ? indicatorOrbState(status) : undefined;
  return (
    <div className="extensionView progressView">
      {orbState === 'listening' ? <IndicatorMicTrace /> : null}
      {showSummary ? (
        <div
          className="progressOverview"
          data-status={normalizedStatus}
          data-mark={animateSummaryText ? 'true' : undefined}
          role={normalizedStatus === 'error' ? 'alert' : 'status'}
        >
          {orbState ? <IndicatorOrb state={orbState} /> : null}
          {animateSummaryText && !orbState ? (
            <IndicatorStatusIcon
              kind={indicatorStatusIconKind(normalizedStatus)}
            />
          ) : null}
          <div>
            <strong>
              <IndicatorSummaryText
                value={summaryLabel}
                animate={animateSummaryText}
              />
            </strong>
            {amount ? (
              <small>
                {animateSummaryText ? (
                  <MorphingIndicatorText value={progressLabel} />
                ) : (
                  progressLabel
                )}
              </small>
            ) : null}
          </div>
          {amount ? (
            <div
              className="progressBar"
              role="progressbar"
              aria-valuenow={amount.value}
              aria-valuemin={0}
              aria-valuemax={amount.total}
              aria-label={status || label || 'Progress'}
            >
              <span style={{ width: `${percent}%` }} />
            </div>
          ) : null}
          {detail ? <p className="progressDetail">{detail}</p> : null}
        </div>
      ) : null}
      {steps.length ? (
        <div className="progressSteps">
          {steps.map((step, index) => (
            <ProgressStepRow key={index} step={step} />
          ))}
        </div>
      ) : null}
      {actions}
    </div>
  );
}
