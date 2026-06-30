import { Router, Request, Response, IRouter } from 'express';
import Stripe from 'stripe';
import { config } from '../config';
import { pool } from '../db/postgres';

const stripe = new Stripe(config.stripeSecretKey || 'sk_placeholder');

export const stripeRouter: IRouter = Router();

stripeRouter.post('/', async (req: Request, res: Response): Promise<void> => {
  const sig = req.headers['stripe-signature'];
  if (!sig || !config.stripeWebhookSecret) {
    res.status(400).json({ error: 'Missing signature or secret' });
    return;
  }

  let event: Stripe.Event;
  try {
    event = stripe.webhooks.constructEvent(req.body as Buffer, sig, config.stripeWebhookSecret);
  } catch (err) {
    res.status(400).json({ error: `Webhook signature verification failed: ${String(err)}` });
    return;
  }

  try {
    switch (event.type) {
      case 'customer.subscription.created':
      case 'customer.subscription.updated':
        await handleSubscriptionUpsert(event.data.object as Stripe.Subscription);
        break;

      case 'customer.subscription.deleted':
        await handleSubscriptionDeleted(event.data.object as Stripe.Subscription);
        break;

      case 'invoice.payment_succeeded':
        await handlePaymentSucceeded(event.data.object as Stripe.Invoice);
        break;

      case 'invoice.payment_failed':
        await handlePaymentFailed(event.data.object as Stripe.Invoice);
        break;

      default:
        // Unhandled event types are acknowledged but not processed.
        break;
    }

    res.json({ received: true });
  } catch (err) {
    console.error('Stripe webhook processing error', err);
    res.status(500).json({ error: 'Webhook processing failed' });
  }
});

async function handleSubscriptionUpsert(sub: Stripe.Subscription): Promise<void> {
  const customerId = typeof sub.customer === 'string' ? sub.customer : sub.customer.id;
  const priceId = sub.items.data[0]?.price.id ?? '';
  const plan = resolvePlan(priceId);

  await pool.query(
    `UPDATE users SET plan = $1, updated_at = now() WHERE stripe_customer_id = $2`,
    [plan, customerId],
  );

  await pool.query(
    `INSERT INTO subscriptions
       (user_id, stripe_subscription_id, stripe_price_id, status,
        current_period_start, current_period_end, cancel_at_period_end)
     SELECT id, $2, $3, $4, $5, $6, $7 FROM users WHERE stripe_customer_id = $1
     ON CONFLICT (stripe_subscription_id) DO UPDATE
       SET status = EXCLUDED.status,
           current_period_start = EXCLUDED.current_period_start,
           current_period_end = EXCLUDED.current_period_end,
           cancel_at_period_end = EXCLUDED.cancel_at_period_end,
           updated_at = now()`,
    [
      customerId,
      sub.id,
      priceId,
      sub.status,
      new Date((sub.current_period_start as number) * 1000),
      new Date((sub.current_period_end as number) * 1000),
      sub.cancel_at_period_end,
    ],
  );
}

async function handleSubscriptionDeleted(sub: Stripe.Subscription): Promise<void> {
  const customerId = typeof sub.customer === 'string' ? sub.customer : sub.customer.id;
  await pool.query(
    `UPDATE users SET plan = 'free', updated_at = now() WHERE stripe_customer_id = $1`,
    [customerId],
  );
  await pool.query(
    `UPDATE subscriptions SET status = 'canceled', updated_at = now()
     WHERE stripe_subscription_id = $1`,
    [sub.id],
  );
}

async function handlePaymentSucceeded(invoice: Stripe.Invoice): Promise<void> {
  const customerId =
    typeof invoice.customer === 'string' ? invoice.customer : invoice.customer?.id ?? '';
  if (!customerId) return;
  const subscriptionId =
    typeof invoice.subscription === 'string'
      ? invoice.subscription
      : invoice.subscription?.id ?? '';
  if (subscriptionId) {
    await pool.query(
      `UPDATE subscriptions SET status = 'active', updated_at = now()
       WHERE stripe_subscription_id = $1`,
      [subscriptionId],
    );
  }
}

async function handlePaymentFailed(invoice: Stripe.Invoice): Promise<void> {
  const subscriptionId =
    typeof invoice.subscription === 'string'
      ? invoice.subscription
      : invoice.subscription?.id ?? '';
  if (subscriptionId) {
    await pool.query(
      `UPDATE subscriptions SET status = 'past_due', updated_at = now()
       WHERE stripe_subscription_id = $1`,
      [subscriptionId],
    );
  }
}

function resolvePlan(priceId: string): string {
  // Price IDs come from env at runtime; this mapping is intentionally loose
  // so adding a new tier only requires a new env var, not a code change.
  if (priceId.includes('pro')) return 'pro';
  if (priceId.includes('team')) return 'team';
  return 'pro'; // default paid tier when price ID doesn't contain a hint
}
