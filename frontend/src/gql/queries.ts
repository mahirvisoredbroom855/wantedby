import { gql } from '@apollo/client';

export const CLUSTER_FIELDS = gql`
  fragment ClusterFields on Cluster {
    id
    name
    category
    gapScore
    frequency
    intensityAvg
    trendScore
    competitorGap
    postCount
    createdAt
    updatedAt
    isSaved
  }
`;

export const GET_CLUSTERS = gql`
  ${CLUSTER_FIELDS}
  query GetClusters($filters: ClusterFilters) {
    clusters(filters: $filters) {
      ...ClusterFields
    }
  }
`;

export const GET_TRENDING = gql`
  ${CLUSTER_FIELDS}
  query GetTrending($limit: Int) {
    trending(limit: $limit) {
      ...ClusterFields
    }
  }
`;

export const GET_CLUSTER = gql`
  ${CLUSTER_FIELDS}
  query GetCluster($id: ID!) {
    cluster(id: $id) {
      ...ClusterFields
      posts {
        id
        platform
        title
        body
        url
        score
        painSummary
        intensity
        createdAt
      }
      competitors {
        id
        name
        url
        source
      }
    }
  }
`;

export const GET_ME = gql`
  query GetMe {
    me {
      id
      email
      name
      plan
      savedClusters {
        id
        name
        category
        gapScore
        isSaved
      }
      alertConfigs {
        id
        category
        minGapScore
        emailEnabled
      }
    }
  }
`;

export const SAVE_CLUSTER = gql`
  mutation SaveCluster($clusterId: ID!) {
    saveCluster(input: { clusterId: $clusterId }) {
      id
      isSaved
    }
  }
`;

export const UNSAVE_CLUSTER = gql`
  mutation UnsaveCluster($clusterId: ID!) {
    unsaveCluster(clusterId: $clusterId)
  }
`;

export const LIVE_FEED_SUBSCRIPTION = gql`
  subscription LiveFeed {
    liveFeed {
      platformId
      platform
      title
      category
      isPainSignal
      confidence
      intensity
      painSummary
      classifiedAt
    }
  }
`;
