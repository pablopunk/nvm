import type { ChangeEvent, ReactNode } from 'react';
import type { AdminModelConfig, AdminSingleRoute, AdminTextRoute } from '../../lib/admin-model-config';
import { modelProviderMayTrainOnPrompts, PROVIDER_TRAINS_ON_PROMPTS_LABEL } from '../../lib/model-data-use';
import { useDashboardQuery } from '../../lib/use-dashboard-query';
import { ConsoleCombobox } from '../ConsoleCombobox';
import { ConsoleSelect } from '../ConsoleSelect';
import { AdminModelRouteCard, type AdminModelTier } from './AdminModelRouteCard';
import { AdminModelsSkeletonRows } from './AdminModelsSkeletonRows';
import { DashboardQueryError } from './DashboardQueryError';
import { ADMIN_MODEL_CONFIG_URL, useAdminRouteSave } from './use-admin-route-save';

const MODEL_CONFIG_REFRESH_MS = 300_000;

type PrimaryPricing = { inputUsdPerMtok: number | null; outputUsdPerMtok: number | null };

const usd = new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD', minimumFractionDigits: 2, maximumFractionDigits: 2 });

function perMillion(value: number | null) {
  return value == null ? '—' : `${usd.format(value)}/M`;
}

function splitModelRef(ref: string) {
  const separator = ref.indexOf('/');
  return { provider: ref.slice(0, separator), modelId: ref.slice(separator + 1) };
}

function tierForSlot(slot: string): AdminModelTier {
  return slot.startsWith('pro') ? 'pro' : 'free';
}

function RoutePricing({ slot, modelRef }: { slot: string; modelRef: string }) {
  const query = useDashboardQuery<{ primaryPricing: PrimaryPricing | null }>(`/api/admin/model-providers?${new URLSearchParams({ slot, modelId: splitModelRef(modelRef).modelId })}`, { refetchInterval: MODEL_CONFIG_REFRESH_MS });
  if (!query.data) return query.isError ? 'Pricing unknown' : null;
  const pricing = query.data.primaryPricing;
  return pricing ? `Input ${perMillion(pricing.inputUsdPerMtok)} · Output ${perMillion(pricing.outputUsdPerMtok)}` : 'Pricing unknown';
}

function RouteDetails({ modelRef, children }: { modelRef: string | null; children: ReactNode }) {
  if (!modelRef || !modelProviderMayTrainOnPrompts(modelRef)) return <>{children}</>;
  return <><strong className="model-route-row__training">{PROVIDER_TRAINS_ON_PROMPTS_LABEL}</strong><br />{children}</>;
}

function TextRouteCard({ route, thinkingLevels }: { route: AdminTextRoute; thinkingLevels: readonly string[] }) {
  const { status, saving, save } = useAdminRouteSave('/api/admin/model');
  const smart = route.kind === 'smart';
  function saveRoute(modelRef: string, thinkingLevel: string) {
    void save(
      { purpose: route.slot, modelRef, thinkingLevel: smart ? thinkingLevel : 'off' },
      (config) => ({ ...config, textRoutes: config.textRoutes.map((item) => item.slot === route.slot ? { ...item, ref: modelRef, thinkingLevel: thinkingLevel as AdminTextRoute['thinkingLevel'] } : item) }),
    );
  }
  function chooseModel(modelRef: string) { saveRoute(modelRef, route.thinkingLevel); }
  function chooseThinking(event: ChangeEvent<HTMLSelectElement>) { if (route.ref) saveRoute(route.ref, event.currentTarget.value); }
  return (
    <AdminModelRouteCard title={route.title} tier={tierForSlot(route.slot)} status={status} details={<RouteDetails modelRef={route.ref}>{route.ref ? <RoutePricing slot={route.slot} modelRef={route.ref} /> : null}</RouteDetails>}>
      <label className="field">Model<ConsoleCombobox label={`${route.title} model`} value={route.ref ?? ''} options={route.options} disabled={saving} onValueChange={chooseModel} /></label>
      <label className="field">Thinking{smart
        ? <ConsoleSelect className="thinking-select" value={route.thinkingLevel} disabled={saving || !route.ref} onChange={chooseThinking} options={thinkingLevels.map((level) => ({ value: level, label: level }))} />
        : <ConsoleSelect className="thinking-select" value="off" disabled options={[{ value: 'off', label: 'off' }]} />}</label>
    </AdminModelRouteCard>
  );
}

function SingleRouteCard({ title, tier, path, route, details, select }: { title: string; tier: AdminModelTier; path: string; route: AdminSingleRoute; details: string; select: (config: AdminModelConfig, ref: string) => AdminModelConfig }) {
  const { status, saving, save } = useAdminRouteSave(path);
  function chooseModel(modelRef: string) {
    void save(splitModelRef(modelRef), (config) => select(config, modelRef));
  }
  return (
    <AdminModelRouteCard title={title} tier={tier} status={status} details={<RouteDetails modelRef={route.ref}>{details}</RouteDetails>}>
      <label className="field">Model<ConsoleCombobox label={`${title} model`} value={route.ref ?? ''} options={route.options} disabled={saving} onValueChange={chooseModel} /></label>
    </AdminModelRouteCard>
  );
}

function selectDictation(config: AdminModelConfig, ref: string): AdminModelConfig {
  return { ...config, dictation: { ...config.dictation, ref } };
}

function selectDecisions(config: AdminModelConfig, ref: string): AdminModelConfig {
  return { ...config, decisions: { ...config.decisions, ref } };
}

export function AdminModelsPanel() {
  const query = useDashboardQuery<AdminModelConfig>(ADMIN_MODEL_CONFIG_URL, { refetchInterval: MODEL_CONFIG_REFRESH_MS });
  if (!query.data) return query.isError ? <DashboardQueryError what="model configuration" retry={query.refetch} /> : <AdminModelsSkeletonRows />;
  const { textRoutes, dictation, decisions, thinkingLevels } = query.data;
  return <>
    {textRoutes.map((route) => <TextRouteCard key={route.slot} route={route} thinkingLevels={thinkingLevels} />)}
    <SingleRouteCard title="Dictation" tier="dictation" path="/api/admin/audio-model" route={dictation} select={selectDictation} details="Budget-checked for a one-minute recording; longer ones can exceed it." />
    <SingleRouteCard title="Decisions" tier="decisions" path="/api/admin/decision-model" route={decisions} select={selectDecisions} details="Only image-capable models." />
  </>;
}
