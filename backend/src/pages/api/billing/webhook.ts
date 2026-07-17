import type { APIRoute } from 'astro';
import { BillingConfigError, constructStripeWebhookEvent, processStripeEvent } from '../../../lib/billing';
import { log } from '../../../lib/log';

export const prerender = false;

export const OPTIONS: APIRoute = async () => new Response(null, { status: 404 });

export const POST: APIRoute = async ({ request }) => {
  const payload = await request.text();

  let event;
  try {
    event = constructStripeWebhookEvent(payload, request.headers.get('stripe-signature'));
  } catch (error) {
    if (error instanceof BillingConfigError) {
      return Response.json({ error: { type: 'billing_not_configured', message: error.message } }, { status: 503 });
    }
    log.warn('stripe_webhook_rejected', { error });
    return Response.json({ error: { type: 'invalid_webhook', message: 'Invalid Stripe webhook signature or payload' } }, { status: 400 });
  }

  try {
    const result = await processStripeEvent(event);
    return Response.json({ received: true, processed: result.processed });
  } catch (error) {
    log.error('stripe_webhook_processing_failed', { error });
    return Response.json({ error: { type: 'server_error', message: 'Failed to process webhook' } }, { status: 500 });
  }
};
