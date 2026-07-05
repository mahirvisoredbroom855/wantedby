import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
} from 'recharts';

interface DataPoint {
  date: string;
  gapScore: number;
  postCount: number;
}

interface Props {
  data: DataPoint[];
}

export function TrendChart({ data }: Props) {
  if (data.length === 0) {
    return <div className="chart-empty">No trend data yet</div>;
  }

  return (
    <ResponsiveContainer width="100%" height={180}>
      <LineChart data={data} margin={{ top: 4, right: 8, bottom: 0, left: -20 }}>
        <XAxis
          dataKey="date"
          tick={{ fill: '#9c8f82', fontSize: 11 }}
          tickLine={false}
          axisLine={false}
        />
        <YAxis
          tick={{ fill: '#9c8f82', fontSize: 11 }}
          tickLine={false}
          axisLine={false}
          domain={[0, 10]}
        />
        <Tooltip
          contentStyle={{ background: '#1a1612', border: '1px solid #2d2620', color: '#f5f0e8' }}
          labelStyle={{ color: '#9c8f82' }}
        />
        <Line
          type="monotone"
          dataKey="gapScore"
          stroke="#E8853A"
          strokeWidth={2}
          dot={false}
          name="Gap Score"
        />
      </LineChart>
    </ResponsiveContainer>
  );
}
