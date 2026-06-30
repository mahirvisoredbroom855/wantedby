import { MongoClient, Db } from 'mongodb';
import { config } from '../config';

let _client: MongoClient | null = null;

export async function getMongoDb(): Promise<Db> {
  if (!_client) {
    _client = new MongoClient(config.mongoUri);
    await _client.connect();
  }
  return _client.db('wantedby');
}

export async function closeMongo(): Promise<void> {
  if (_client) {
    await _client.close();
    _client = null;
  }
}
