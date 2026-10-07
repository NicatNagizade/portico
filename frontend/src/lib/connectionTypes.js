export const CONNECTION_TYPES = [
  { value: 'mysql', label: 'MySQL' },
  { value: 'postgres', label: 'Postgres' },
  { value: 'sqlite', label: 'SQLite' },
  { value: 'typesense', label: 'Typesense' },
  { value: 'mongodb', label: 'MongoDB' },
  { value: 'redis', label: 'Redis' },
]

const MYSQL_FIELDS = [
  { key: 'host', label: 'Host', type: 'text', required: true, defaultValue: 'localhost' },
  { key: 'port', label: 'Port', type: 'number', required: true, defaultValue: 3306 },
  { key: 'user', label: 'User', type: 'text', required: true, defaultValue: '' },
  { key: 'password', label: 'Password', type: 'password', required: false, defaultValue: '' },
  { key: 'database', label: 'Database', type: 'text', required: true, defaultValue: '' },
]

const POSTGRES_FIELDS = [
  { key: 'host', label: 'Host', type: 'text', required: true, defaultValue: 'localhost' },
  { key: 'port', label: 'Port', type: 'number', required: true, defaultValue: 5432 },
  { key: 'user', label: 'User', type: 'text', required: true, defaultValue: '' },
  { key: 'password', label: 'Password', type: 'password', required: false, defaultValue: '' },
  { key: 'database', label: 'Database', type: 'text', required: true, defaultValue: '' },
  { key: 'sslmode', label: 'SSL mode', type: 'text', required: false, defaultValue: 'disable' },
]

const SQLITE_FIELDS = [
  { key: 'path', label: 'Database path', type: 'text', required: true, defaultValue: './data.db' },
]

const TYPESENSE_FIELDS = [
  { key: 'host', label: 'Host', type: 'text', required: true, defaultValue: 'localhost' },
  { key: 'port', label: 'Port', type: 'number', required: true, defaultValue: 8108 },
  { key: 'api_key', label: 'API key', type: 'password', required: true, defaultValue: '' },
  { key: 'protocol', label: 'Protocol', type: 'text', required: true, defaultValue: 'http' },
]

const MONGODB_FIELDS = [
  { key: 'host', label: 'Host', type: 'text', required: true, defaultValue: '127.0.0.1' },
  { key: 'port', label: 'Port', type: 'number', required: true, defaultValue: 27017 },
  { key: 'user', label: 'User', type: 'text', required: false, defaultValue: '' },
  { key: 'password', label: 'Password', type: 'password', required: false, defaultValue: '' },
  { key: 'database', label: 'Database', type: 'text', required: true, defaultValue: '' },
  { key: 'auth_source', label: 'Auth source', type: 'text', required: false, defaultValue: 'admin' },
]

const REDIS_FIELDS = [
  { key: 'host', label: 'Host', type: 'text', required: true, defaultValue: '127.0.0.1' },
  { key: 'port', label: 'Port', type: 'number', required: true, defaultValue: 6379 },
  { key: 'password', label: 'Password', type: 'password', required: false, defaultValue: '' },
  { key: 'db', label: 'DB index', type: 'number', required: false, defaultValue: 0 },
]

export function getConfigFields(type) {
  switch (type) {
    case 'mysql':
      return MYSQL_FIELDS
    case 'postgres':
      return POSTGRES_FIELDS
    case 'sqlite':
      return SQLITE_FIELDS
    case 'typesense':
      return TYPESENSE_FIELDS
    case 'mongodb':
      return MONGODB_FIELDS
    case 'redis':
      return REDIS_FIELDS
    default:
      return null
  }
}

export function defaultConfigForType(type) {
  const fields = getConfigFields(type)
  if (!fields) return {}
  return Object.fromEntries(fields.map((f) => [f.key, f.defaultValue]))
}

export function parseConfig(config) {
  if (!config) return {}
  if (typeof config === 'string') {
    try {
      return JSON.parse(config)
    } catch {
      return {}
    }
  }
  return { ...config }
}
