import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { MockedProvider } from '@apollo/client/testing/react';
import { ClusterCard } from '../components/ClusterCard';
import type { Cluster } from '../types';

const cluster: Cluster = {
  id: 'abc123',
  name: 'Freelancer Invoice Pain',
  category: 'finance',
  gapScore: 7.8,
  frequency: 12,
  intensityAvg: 8.2,
  trendScore: 6.5,
  competitorGap: 5.0,
  postCount: 12,
  createdAt: new Date().toISOString(),
  updatedAt: new Date().toISOString(),
  isSaved: false,
};

describe('ClusterCard', () => {
  it('renders cluster name and category', () => {
    render(
      <MockedProvider mocks={[]} >
        <ClusterCard cluster={cluster} />
      </MockedProvider>,
    );
    expect(screen.getByText('Freelancer Invoice Pain')).toBeInTheDocument();
    expect(screen.getByText('finance')).toBeInTheDocument();
  });

  it('calls onClick when card is clicked', () => {
    const handleClick = vi.fn();
    render(
      <MockedProvider mocks={[]} >
        <ClusterCard cluster={cluster} onClick={handleClick} />
      </MockedProvider>,
    );
    // The card div has role=button and the bookmark button is also a button —
    // click the card (first in DOM order).
    fireEvent.click(screen.getAllByRole('button')[0]);
    expect(handleClick).toHaveBeenCalledOnce();
  });

  it('shows unsaved bookmark icon when isSaved=false', () => {
    render(
      <MockedProvider mocks={[]} >
        <ClusterCard cluster={cluster} />
      </MockedProvider>,
    );
    expect(screen.getByLabelText('Save cluster')).toBeInTheDocument();
  });

  it('shows saved bookmark icon when isSaved=true', () => {
    render(
      <MockedProvider mocks={[]} >
        <ClusterCard cluster={{ ...cluster, isSaved: true }} />
      </MockedProvider>,
    );
    expect(screen.getByLabelText('Unsave cluster')).toBeInTheDocument();
  });
});
