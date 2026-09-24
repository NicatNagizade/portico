export const CONNECTION_TYPES = [
  { value: 'mysql', label: 'MySQL' },
  { value: 'postgres', label: 'Postgres' },
  { value: 'typesense', label: 'Typesense' },
  { value: 'mongodb', label: 'MongoDB' },
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

export function getConfigFields(type) {
  switch (type) {
    case 'mysql':
      return MYSQL_FIELDS
    case 'postgres':
      return POSTGRES_FIELDS
    case 'typesense':
      return TYPESENSE_FIELDS
    case 'mongodb':
      return MONGODB_FIELDS
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
