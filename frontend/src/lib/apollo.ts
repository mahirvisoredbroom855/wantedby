import { ApolloClient, InMemoryCache, createHttpLink, split } from '@apollo/client';
import { setContext } from '@apollo/client/link/context';
import { GraphQLWsLink } from '@apollo/client/link/subscriptions';
import { getMainDefinition } from '@apollo/client/utilities';
import { createClient } from 'graphql-ws';

const API_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:4000';
const WS_URL = API_URL.replace(/^http/, 'ws') + '/graphql';

const httpLink = createHttpLink({ uri: `${API_URL}/graphql` });

// Token getter is set once the Auth0 context is ready.
let _getAccessToken: (() => Promise<string>) | null = null;

export function setTokenGetter(fn: () => Promise<string>) {
  _getAccessToken = fn;
}

const authLink = setContext(async (_, { headers }) => {
  if (!_getAccessToken) return { headers };
  try {
    const token = await _getAccessToken();
    return { headers: { ...headers, authorization: `Bearer ${token}` } };
  } catch {
    return { headers };
  }
});

const wsLink = new GraphQLWsLink(
  createClient({
    url: WS_URL,
    connectionParams: async () => {
      if (!_getAccessToken) return {};
      try {
        const token = await _getAccessToken();
        return { authorization: `Bearer ${token}` };
      } catch {
        return {};
      }
    },
  }),
);

const splitLink = split(
  ({ query }) => {
    const def = getMainDefinition(query);
    return def.kind === 'OperationDefinition' && def.operation === 'subscription';
  },
  wsLink,
  authLink.concat(httpLink),
);

export const apolloClient = new ApolloClient({
  link: splitLink,
  cache: new InMemoryCache({
    typePolicies: {
      Query: {
        fields: {
          clusters: { merge: false },
        },
      },
    },
  }),
});
