import { GraphQLScalarType, Kind } from 'graphql';
import { clusterResolvers } from './clusters';
import { userResolvers } from './users';
import { pubsub, LIVE_FEED_TOPIC, CLUSTER_UPDATED_TOPIC } from '../../websocket/pubsub';

const DateTimeScalar = new GraphQLScalarType({
  name: 'DateTime',
  serialize: (value: unknown) => (value instanceof Date ? value.toISOString() : String(value)),
  parseValue: (value: unknown) => new Date(String(value)),
  parseLiteral: (ast) => (ast.kind === Kind.STRING ? new Date(ast.value) : null),
});

export const resolvers = {
  DateTime: DateTimeScalar,

  Query: {
    ...clusterResolvers.Query,
    ...userResolvers.Query,
  },

  Mutation: {
    ...clusterResolvers.Mutation,
    ...userResolvers.Mutation,
  },

  Subscription: {
    liveFeed: {
      subscribe: () => pubsub.asyncIterableIterator(LIVE_FEED_TOPIC),
    },
    clusterUpdated: {
      subscribe: () => pubsub.asyncIterableIterator(CLUSTER_UPDATED_TOPIC),
    },
  },

  Cluster: clusterResolvers.Cluster,
  User: userResolvers.User,
};
