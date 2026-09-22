import { useEffect, useMemo, useState } from 'react'
import { checkConnection } from '../../api/connections'
import {
  CONNECTION_TYPES,
  defaultConfigForType,
  getConfigFields,
  parseConfig,
} from '../../lib/connectionTypes'
import { Field, PrimaryButton, SecondaryButton, inputClassName } from '../ui'

export default function ConnectionForm({ initial, onSubmit, busy, submitLabel }) {
  const [name, setName] = useState(initial?.name || '')
  const [type, setType] = useState(initial?.type || 'mysql')
  const [config, setConfig] = useState(() => {
    const parsed = parseConfig(initial?.config)
    return Object.keys(parsed).length ? parsed : defaultConfigForType(initial?.type || 'mysql')
  })
  const [rawJson, setRawJson] = useState(() =>
    JSON.stringify(parseConfig(initial?.config) || {}, null, 2),
  )
  const [jsonError, setJsonError] = useState('')
  const [checking, setChecking] = useState(false)
  const [checkMessage, setCheckMessage] = useState('')
  const [checkOk, setCheckOk] = useState(false)

  const fields = useMemo(() => getConfigFields(type), [type])
  const useRawJson = !fields

  useEffect(() => {
    if (!initial) {
      setConfig(defaultConfigForType(type))
      setRawJson(JSON.stringify(defaultConfigForType(type), null, 2))
      setJsonError('')
    }
  }, [type, initial])

  function updateConfigField(key, value, fieldType) {
    setConfig((prev) => ({
      ...prev,
      [key]: fieldType === 'number' ? (value === '' ? '' : Number(value)) : value,
    }))
  }

  function buildConfig() {
    if (!useRawJson) {
      setJsonError('')
      return config
    }
    try {
      const nextConfig = JSON.parse(rawJson)
      setJsonError('')
      return nextConfig
    } catch {
      setJsonError('Config must be valid JSON')
      return null
    }
  }

  function handleSubmit(event) {
    event.preventDefault()
    const nextConfig = buildConfig()
    if (!nextConfig) return

    onSubmit({
      name: name.trim(),
      type,
      config: nextConfig,
    })
  }

  async function handleCheck() {
    const nextConfig = buildConfig()
    if (!nextConfig) return

    setChecking(true)
    setCheckMessage('')
    setCheckOk(false)
    try {
      const body = { type, config: nextConfig }
      if (initial?.id) body.id = initial.id
      await checkConnection(body)
      setCheckOk(true)
      setCheckMessage('Connection successful.')
    } catch (err) {
      setCheckOk(false)
      setCheckMessage(err.message || 'Connection check failed')
    } finally {
      setChecking(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-6">
      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="Name">
          <input
            required
            className={inputClassName}
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="mysql-app"
          />
        </Field>
        <Field label="Type">
          <select
            className={inputClassName}
            value={type}
            onChange={(e) => setType(e.target.value)}
            disabled={Boolean(initial)}
          >
            {CONNECTION_TYPES.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
        </Field>
      </div>

      <div className="rounded-xl border border-[var(--border)] bg-[var(--bg-elevated)]/70 p-4">
        <div className="mb-4 flex items-center gap-2">
          <span className="flex h-6 w-6 items-center justify-center rounded-md bg-[var(--accent)] font-mono text-[10px] font-bold text-white">
            01
          </span>
          <div>
            <h3 className="text-sm font-semibold text-[var(--text)]">Connector config</h3>
            <p className="text-xs text-[var(--text-muted)]">
              Fields change based on the connection type.
            </p>
          </div>
        </div>
        {useRawJson ? (
          <Field label="JSON config" hint="Used for types without a typed form (e.g. MongoDB).">
            <textarea
              className={`${inputClassName} min-h-40 font-mono text-xs`}
              value={rawJson}
              onChange={(e) => setRawJson(e.target.value)}
            />
            {jsonError ? <p className="mt-1 text-xs text-[var(--danger)]">{jsonError}</p> : null}
          </Field>
        ) : (
          <div className="grid gap-4 sm:grid-cols-2">
            {fields.map((field) => {
              const isSecret = field.type === 'password'
              const keepExisting = Boolean(initial) && isSecret
              return (
                <Field
                  key={field.key}
                  label={field.label}
                  hint={keepExisting ? 'Leave blank to keep the current value.' : undefined}
                >
                  <input
                    className={inputClassName}
                    type={
                      isSecret ? 'password' : field.type === 'number' ? 'number' : 'text'
                    }
                    required={field.required && !keepExisting}
                    autoComplete={isSecret ? 'new-password' : undefined}
                    placeholder={keepExisting ? 'Unchanged' : undefined}
                    value={config[field.key] ?? ''}
                    onChange={(e) => updateConfigField(field.key, e.target.value, field.type)}
                  />
                </Field>
              )
            })}
          </div>
        )}
      </div>

      {checkMessage ? (
        <p
          className={`text-sm ${checkOk ? 'text-[var(--success)]' : 'text-[var(--danger)]'}`}
          role="status"
        >
          {checkMessage}
        </p>
      ) : null}

      <div className="flex flex-wrap items-center justify-end gap-2 border-t border-[var(--border)] pt-5">
        <SecondaryButton type="button" disabled={busy || checking} onClick={handleCheck}>
          {checking ? 'Checking…' : 'Check connection'}
        </SecondaryButton>
        <PrimaryButton type="submit" disabled={busy || checking}>
          {busy ? 'Saving…' : submitLabel}
        </PrimaryButton>
      </div>
    </form>
  )
}
