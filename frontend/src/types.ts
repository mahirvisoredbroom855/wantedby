export interface Cluster {
  id: string;
  name: string;
  category: string;
  gapScore: number;
  frequency: number;
  intensityAvg: number;
  trendScore: number;
  competitorGap: number;
  postCount: number;
  createdAt: string;
  updatedAt: string;
  isSaved: boolean;
}

export interface ClusterPost {
  id: string;
  platform: string;
  title: string;
  body?: string;
  url?: string;
  score: number;
  painSummary?: string;
  intensity?: number;
  createdAt: string;
}

export interface Competitor {
  id: string;
  name: string;
  url?: string;
  source: string;
}

export interface LivePost {
  platformId: string;
  platform: string;
  title: string;
  category?: string;
  isPainSignal: boolean;
  confidence?: number;
  intensity?: number;
  painSummary?: string;
  classifiedAt?: string;
}
