/**
 * Stripe webhook handler tests — stripe.webhooks.constructEvent is mocked so
 * no real Stripe calls are made. Verifies routing of the 5 required event types.
 */

// Mock pool so DB calls don't require Postgres.
jest.mock('../db/postgres', () => ({
  pool: { query: jest.fn().mockResolvedValue({ rows: [] }) },
}));

const mockConstructEvent = jest.fn();

jest.mock('stripe', () => {
  return jest.fn().mockImplementation(() => ({
    webhooks: { constructEvent: mockConstructEvent },
  }));
});

import request from 'supertest';
import express from 'express';
import { raw } from 'body-parser';
import type Stripe from 'stripe';
import { stripeRouter } from '../webhooks/stripe';

const app = express();
app.use('/webhooks/stripe', raw({ type: 'application/json' }));
app.use('/webhooks/stripe', stripeRouter);

function makeSubscription(status = 'active'): Stripe.Subscription {
  return {
    id: 'sub_test',
    customer: 'cus_test',
    status,
    items: { data: [{ price: { id: 'price_pro_monthly' } }] },
    current_period_start: 1700000000,
    current_period_end: 1702592000,
    cancel_at_period_end: false,
  } as unknown as Stripe.Subscription;
}

function makeInvoice(subId = 'sub_test'): Stripe.Invoice {
  return {
    customer: 'cus_test',
    subscription: subId,
  } as unknown as Stripe.Invoice;
}

describe('Stripe webhook router', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('returns 400 when stripe-signature header is missing', async () => {
    const res = await request(app)
      .post('/webhooks/stripe')
      .set('Content-Type', 'application/json')
      .send('{}');
    expect(res.status).toBe(400);
  });

  it('handles customer.subscription.created', async () => {
    mockConstructEvent.mockReturnValueOnce({
      type: 'customer.subscription.created',
      data: { object: makeSubscription() },
    });

    const res = await request(app)
      .post('/webhooks/stripe')
      .set('stripe-signature', 'sig_test')
      .set('Content-Type', 'application/json')
      .send('{}');

    expect(res.status).toBe(200);
    expect(res.body).toEqual({ received: true });
  });

  it('handles customer.subscription.deleted', async () => {
    mockConstructEvent.mockReturnValueOnce({
      type: 'customer.subscription.deleted',
      data: { object: makeSubscription('canceled') },
    });

    const res = await request(app)
      .post('/webhooks/stripe')
      .set('stripe-signature', 'sig_test')
      .set('Content-Type', 'application/json')
      .send('{}');

    expect(res.status).toBe(200);
  });

  it('handles invoice.payment_succeeded', async () => {
    mockConstructEvent.mockReturnValueOnce({
      type: 'invoice.payment_succeeded',
      data: { object: makeInvoice() },
    });

    const res = await request(app)
      .post('/webhooks/stripe')
      .set('stripe-signature', 'sig_test')
      .set('Content-Type', 'application/json')
      .send('{}');

    expect(res.status).toBe(200);
  });

  it('handles invoice.payment_failed', async () => {
    mockConstructEvent.mockReturnValueOnce({
      type: 'invoice.payment_failed',
      data: { object: makeInvoice() },
    });

    const res = await request(app)
      .post('/webhooks/stripe')
      .set('stripe-signature', 'sig_test')
      .set('Content-Type', 'application/json')
      .send('{}');

    expect(res.status).toBe(200);
  });

  it('acknowledges unhandled event types without error', async () => {
    mockConstructEvent.mockReturnValueOnce({
      type: 'charge.updated',
      data: { object: {} },
    });

    const res = await request(app)
      .post('/webhooks/stripe')
      .set('stripe-signature', 'sig_test')
      .set('Content-Type', 'application/json')
      .send('{}');

    expect(res.status).toBe(200);
    expect(res.body).toEqual({ received: true });
  });
});
