export const typeDefs = `#graphql

  scalar DateTime

  type Cluster {
    id: ID!
    name: String!
    category: String!
    gapScore: Float!
    frequency: Int!
    intensityAvg: Float!
    trendScore: Float!
    competitorGap: Float!
    postCount: Int!
    createdAt: DateTime!
    updatedAt: DateTime!
    posts: [ClusterPost!]!
    competitors: [Competitor!]!
    isSaved: Boolean!
  }

  type ClusterPost {
    id: ID!
    mongoClassifiedId: String!
    platform: String!
    platformId: String!
    title: String!
    body: String
    url: String
    score: Int!
    painSummary: String
    intensity: Int
    createdAt: DateTime!
  }

  type Competitor {
    id: ID!
    name: String!
    url: String
    source: String!
  }

  type LivePost {
    platformId: String!
    platform: String!
    title: String!
    body: String
    url: String
    category: String
    isPainSignal: Boolean!
    confidence: Float
    intensity: Int
    painSummary: String
    classifiedAt: DateTime
  }

  type User {
    id: ID!
    auth0Id: String!
    email: String!
    name: String
    plan: String!
    savedClusters: [Cluster!]!
    alertConfigs: [AlertConfig!]!
    apiKeys: [ApiKey!]!
  }

  type AlertConfig {
    id: ID!
    category: String!
    minGapScore: Float!
    emailEnabled: Boolean!
  }

  type ApiKey {
    id: ID!
    name: String!
    lastUsed: DateTime
    createdAt: DateTime!
  }

  type ApiKeyWithSecret {
    id: ID!
    name: String!
    secret: String!
    createdAt: DateTime!
  }

  input ClusterFilters {
    category: String
    minGapScore: Float
    limit: Int
    offset: Int
  }

  input SaveClusterInput {
    clusterId: ID!
  }

  input UpsertAlertInput {
    category: String!
    minGapScore: Float!
    emailEnabled: Boolean!
  }

  input CreateApiKeyInput {
    name: String!
  }

  type Query {
    clusters(filters: ClusterFilters): [Cluster!]!
    cluster(id: ID!): Cluster
    trending(limit: Int): [Cluster!]!
    me: User
  }

  type Mutation {
    saveCluster(input: SaveClusterInput!): Cluster!
    unsaveCluster(clusterId: ID!): Boolean!
    upsertAlert(input: UpsertAlertInput!): AlertConfig!
    deleteAlert(category: String!): Boolean!
    createApiKey(input: CreateApiKeyInput!): ApiKeyWithSecret!
    revokeApiKey(id: ID!): Boolean!
  }

  type Subscription {
    liveFeed: LivePost!
    clusterUpdated: Cluster!
  }
`;
