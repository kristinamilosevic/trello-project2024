const cassandra = require('cassandra-driver');
const logger = require('./logging/logger');
// Konekcija sa Cassandra bazom
const client = new cassandra.Client({
  contactPoints: ['127.0.0.1'], // IP adresa Cassandra servera
  localDataCenter: 'datacenter1', // Ime Data Centra (proveri u konfiguraciji Cassandre)
  keyspace: 'notifications' // Keyspace koji koristiš
});

async function testConnection() {
  try {
    // Test konekcije
    await client.connect();
    logger.info('Connected to Cassandra', {
      service: 'notifications-service',
      action: 'connect',
      status: 'success'
    });

    
    // Testni upit
    const result = await client.execute('SELECT * FROM notifications');
    logger.debug('Fetched rows from notifications table', {
      service: 'notifications-service',
      rowCount: result.rowLength
    });
    
  } catch (error) {
    logger.error('Failed to connect or execute query on Cassandra', {
      service: 'notifications-service',
      error: error.message,
      stack: error.stack
    });
  } finally {
    // Zatvaranje konekcije
    await client.shutdown();
    logger.info('Cassandra connection closed', {
      service: 'notifications-service',
      action: 'shutdown'
    });
  }
}

testConnection();
