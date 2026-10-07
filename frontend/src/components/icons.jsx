export function Icon({ children, size = 16, className }) {
  return (
    <svg className={className} width={size} height={size} viewBox="0 0 24 24" fill="none" aria-hidden="true">
      {children}
    </svg>
  )
}

export function CreateIcon() {
  return (
    <Icon>
      <path d="M12 5v14M5 12h14" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" />
    </Icon>
  )
}

export function OpenIcon() {
  return (
    <Icon>
      <path
        d="M2.5 12s3.5-6.5 9.5-6.5S21.5 12 21.5 12s-3.5 6.5-9.5 6.5S2.5 12 2.5 12Z"
        stroke="currentColor"
        strokeWidth="1.75"
      />
      <circle cx="12" cy="12" r="2.75" stroke="currentColor" strokeWidth="1.75" />
    </Icon>
  )
}

export function EditIcon() {
  return (
    <Icon>
      <path
        d="M4 20h4L18.5 9.5a2.12 2.12 0 0 0-3-3L5 17v3Z"
        stroke="currentColor"
        strokeWidth="1.75"
        strokeLinejoin="round"
      />
      <path d="m13.5 6.5 3 3" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" />
    </Icon>
  )
}

export function DeleteIcon() {
  return (
    <Icon>
      <path
        d="M4 7h16M9 7V5h6v2M6.5 7l.8 13h9.4l.8-13"
        stroke="currentColor"
        strokeWidth="1.75"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </Icon>
  )
}

export function BackIcon() {
  return (
    <Icon>
      <path
        d="M15 6 9 12l6 6"
        stroke="currentColor"
        strokeWidth="1.75"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </Icon>
  )
}

export function PlayIcon() {
  return (
    <Icon>
      <path d="M8 5.5v13l11-6.5-11-6.5Z" fill="currentColor" />
    </Icon>
  )
}

export function SpinnerIcon() {
  return (
    <Icon className="animate-spin">
      <circle cx="12" cy="12" r="8" stroke="currentColor" strokeWidth="2" opacity="0.35" />
      <path d="M12 4a8 8 0 0 1 8 8" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
    </Icon>
  )
}

export function LogsIcon() {
  return (
    <Icon>
      <path d="M5 4h14v16H5V4Z" stroke="currentColor" strokeWidth="1.75" strokeLinejoin="round" />
      <path d="M8 8h8M8 12h8M8 16h5" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" />
    </Icon>
  )
}

export function TrashIcon() {
  return (
    <Icon size={14}>
      <path
        d="M4 7h16M9 7V5h6v2M8 7l1 12h6l1-12"
        stroke="currentColor"
        strokeWidth="1.75"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </Icon>
  )
}
