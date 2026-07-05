interface Props {
  score: number; // 0–10
  size?: 'sm' | 'md';
}

export function GapScoreBar({ score, size = 'md' }: Props) {
  const pct = Math.min(100, (score / 10) * 100);
  const color =
    score >= 7 ? '#E8853A' : score >= 4 ? '#c4832e' : '#6b5c47';

  return (
    <div className={`gap-score-bar ${size}`}>
      <div
        className="gap-score-fill"
        style={{ width: `${pct}%`, backgroundColor: color }}
      />
      <span className="gap-score-label">{score.toFixed(1)}</span>
    </div>
  );
}
