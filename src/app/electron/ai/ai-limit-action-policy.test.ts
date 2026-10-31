import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createAiEventActionRegistrar,
  trustedAiLimitAction,
} from './ai-limit-action-policy';

const dashboardUrl = 'http://localhost:4321/dashboard';

test('accepts only host-defined AI limit actions for the active environment', () => {
  const dashboardAction = {
    type: 'openUrl' as const,
    title: 'Open Dashboard',
    url: dashboardUrl,
  };
  assert.deepEqual(
    trustedAiLimitAction(
      {
        type: 'error',
        data: { kind: 'insufficient_credits', action: dashboardAction },
      },
      dashboardUrl,
    ),
    dashboardAction,
  );
  assert.deepEqual(
    trustedAiLimitAction(
      {
        type: 'error',
        data: {
          kind: 'unsupported_client',
          action: { type: 'checkForUpdates', title: 'Check for Update' },
        },
      },
      dashboardUrl,
    ),
    { type: 'checkForUpdates', title: 'Check for Update' },
  );
});

test('rejects arbitrary or stale AI limit actions', () => {
  for (const event of [
    {
      type: 'error',
      data: {
        kind: 'insufficient_credits',
        action: {
          type: 'openUrl',
          title: 'Open Dashboard',
          url: 'https://www.nvm.fyi/dashboard',
        },
      },
    },
    {
      type: 'error',
      data: {
        kind: 'deprecation_warning',
        action: { type: 'openUrl', title: 'Open', url: dashboardUrl },
      },
    },
    {
      type: 'delta',
      data: {
        kind: 'insufficient_credits',
        action: { type: 'openUrl', title: 'Open', url: dashboardUrl },
      },
    },
  ])
    assert.equal(trustedAiLimitAction(event, dashboardUrl), null);
});

test('registers a trusted action for the renderer and strips untrusted ones', () => {
  const registerViewAction = (action: unknown) => ({
    ...(action as object),
    executionId: 'token-1',
  });
  const register = createAiEventActionRegistrar({
    getDashboardUrl: () => dashboardUrl,
    registerViewAction: registerViewAction as never,
  });
  const dashboardAction = {
    type: 'openUrl' as const,
    title: 'Open Dashboard',
    url: dashboardUrl,
  };

  const trusted = register({
    type: 'error',
    data: { kind: 'insufficient_credits', action: dashboardAction },
  });
  const untrusted = register({
    type: 'error',
    data: {
      kind: 'insufficient_credits',
      action: { ...dashboardAction, url: 'https://evil.example' },
    },
  });
  const noAction = { type: 'delta', data: { text: 'hi' } };

  assert.deepEqual((trusted.data as { action: unknown }).action, {
    ...dashboardAction,
    executionId: 'token-1',
  });
  assert.equal((untrusted.data as { action: unknown }).action, undefined);
  assert.equal(register(noAction), noAction);
});
