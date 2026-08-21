const fs = require('node:fs');
const path = require('node:path');
const { spawn, spawnSync } = require('node:child_process');
const { parseEnv } = require('node:util');

const forwardedEvents = [
  'checkout.session.completed',
  'checkout.session.async_payment_succeeded',
  'invoice.paid',
  'payment_intent.succeeded',
  'customer.subscription.updated',
  'customer.subscription.deleted',
].join(',');

function stripeDevelopmentConfiguration(root) {
  const environmentPath = path.join(root, 'src', 'backend', '.env');
  if (!fs.existsSync(environmentPath)) return null;
  const environment = {
    ...parseEnv(fs.readFileSync(environmentPath, 'utf8')),
    ...process.env,
  };
  if (!environment.STRIPE_SECRET_KEY?.startsWith('sk_test_')) return null;
  if (
    !environment.STRIPE_SUBSCRIPTION_TIERS &&
    !environment.STRIPE_TOP_UP_PACKS
  )
    return null;

  const stripeEnvironment = {
    ...process.env,
    STRIPE_API_KEY: environment.STRIPE_SECRET_KEY,
  };
  const secret = spawnSync(
    'stripe',
    ['listen', '--print-secret', '--skip-update'],
    {
      cwd: root,
      encoding: 'utf8',
      env: stripeEnvironment,
      timeout: 10_000,
    },
  );
  if (secret.error?.code === 'ENOENT') {
    console.warn(
      'Stripe CLI is not installed; local billing webhooks are disabled.',
    );
    return null;
  }
  if (
    secret.status !== 0 ||
    typeof secret.stdout !== 'string' ||
    !secret.stdout.trim().startsWith('whsec_')
  ) {
    console.warn(
      'Stripe CLI could not initialize; local billing webhooks are disabled.',
    );
    return null;
  }
  return {
    environment: stripeEnvironment,
    webhookSecret: secret.stdout.trim(),
  };
}

function startStripeDevelopmentListener(root, isWindows) {
  const configuration = stripeDevelopmentConfiguration(root);
  if (!configuration) return null;
  const listener = spawn(
    'stripe',
    [
      'listen',
      '--skip-update',
      '--events',
      forwardedEvents,
      '--forward-to',
      'http://localhost:4321/api/billing/webhook',
    ],
    {
      cwd: root,
      stdio: ['ignore', 'ignore', 'inherit'],
      shell: isWindows,
      detached: !isWindows,
      env: configuration.environment,
    },
  );
  console.log('Stripe test webhook forwarding enabled.');
  return { listener, webhookSecret: configuration.webhookSecret };
}

module.exports = { startStripeDevelopmentListener };
