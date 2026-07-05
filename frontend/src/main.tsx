import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { Auth0Provider } from '@auth0/auth0-react';
import { ApolloProvider } from '@apollo/client/react';
import { auth0Config } from './lib/auth0Config';
import { apolloClient } from './lib/apollo';
import { App } from './App';
import './index.css';

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <Auth0Provider {...auth0Config}>
      <ApolloProvider client={apolloClient}>
        <App />
      </ApolloProvider>
    </Auth0Provider>
  </StrictMode>,
);
