import { useId } from 'react'
import { inputClassName } from './ui'

/** Text input with optional suggestions via a native datalist. */
export default function AutocompleteInput({ options = [], className = inputClassName, ...props }) {
  const listId = useId()
  const hasOptions = options.length > 0

  return (
    <>
      <input className={className} list={hasOptions ? listId : undefined} {...props} />
      {hasOptions ? (
        <datalist id={listId}>
          {options.map((option) => (
            <option key={option} value={option} />
          ))}
        </datalist>
      ) : null}
    </>
  )
}
