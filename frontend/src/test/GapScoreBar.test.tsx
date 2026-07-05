import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { GapScoreBar } from '../components/GapScoreBar';

describe('GapScoreBar', () => {
  it('renders the score label', () => {
    render(<GapScoreBar score={7.5} />);
    expect(screen.getByText('7.5')).toBeInTheDocument();
  });

  it('caps fill width at 100%', () => {
    const { container } = render(<GapScoreBar score={12} />);
    const fill = container.querySelector('.gap-score-fill') as HTMLElement;
    expect(fill.style.width).toBe('100%');
  });

  it('uses amber color for high scores', () => {
    const { container } = render(<GapScoreBar score={8} />);
    const fill = container.querySelector('.gap-score-fill') as HTMLElement;
    expect(fill.style.backgroundColor).toBe('rgb(232, 133, 58)');
  });

  it('applies sm size class', () => {
    const { container } = render(<GapScoreBar score={5} size="sm" />);
    expect(container.querySelector('.gap-score-bar.sm')).not.toBeNull();
  });
});
