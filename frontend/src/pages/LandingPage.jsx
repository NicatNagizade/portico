import { Link } from 'react-router-dom'

function PorticoMark({ className = '' }) {
  return (
    <div
      className={[
        'flex h-11 w-11 items-center justify-center rounded-2xl bg-[var(--accent)] text-white shadow-[var(--shadow-md)]',
        className,
      ].join(' ')}
    >
      <svg width="22" height="22" viewBox="0 0 24 24" fill="none" aria-hidden="true">
        <path
          d="M4 12c0-4.4 3.6-8 8-8h2v4h-2a4 4 0 0 0 0 8h2v4h-2c-4.4 0-8-3.6-8-8Z"
          fill="currentColor"
          opacity="0.35"
        />
        <path
          d="M20 12c0 4.4-3.6 8-8 8h-2v-4h2a4 4 0 0 0 0-8h-2V4h2c4.4 0 8 3.6 8 8Z"
          fill="currentColor"
        />
      </svg>
    </div>
  )
}

function SyncScene() {
  return (
    <svg
      viewBox="0 0 720 420"
      className="h-auto w-full max-w-3xl"
      role="img"
      aria-label="Diagram of data moving from source databases through Portico into destinations"
    >
      <defs>
        <linearGradient id="scene-glow" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="rgba(11,110,99,0.18)" />
          <stop offset="100%" stopColor="rgba(29,78,216,0.08)" />
        </linearGradient>
        <linearGradient id="flow-line" x1="0%" y1="0%" x2="100%" y2="0%">
          <stop offset="0%" stopColor="var(--accent)" stopOpacity="0.15" />
          <stop offset="50%" stopColor="var(--accent)" stopOpacity="0.85" />
          <stop offset="100%" stopColor="var(--accent)" stopOpacity="0.15" />
        </linearGradient>
      </defs>

      <rect x="24" y="28" width="672" height="364" rx="28" fill="url(#scene-glow)" />
      <rect
        x="24"
        y="28"
        width="672"
        height="364"
        rx="28"
        fill="none"
        stroke="var(--border)"
        strokeWidth="1.5"
      />

      {/* Source stack */}
      <g className="landing-float">
        <rect x="72" y="110" width="150" height="200" rx="18" fill="var(--surface)" stroke="var(--border-strong)" />
        <text x="147" y="142" textAnchor="middle" fill="var(--text-muted)" fontFamily="var(--font-mono)" fontSize="11">
          SOURCES
        </text>
        <rect x="92" y="162" width="110" height="28" rx="8" fill="var(--mysql-soft)" />
        <text x="147" y="181" textAnchor="middle" fill="var(--mysql)" fontFamily="var(--font-mono)" fontSize="12">
          MySQL
        </text>
        <rect x="92" y="202" width="110" height="28" rx="8" fill="var(--postgres-soft)" />
        <text x="147" y="221" textAnchor="middle" fill="var(--postgres)" fontFamily="var(--font-mono)" fontSize="12">
          Postgres
        </text>
        <rect x="92" y="242" width="110" height="28" rx="8" fill="var(--mongodb-soft)" />
        <text x="147" y="261" textAnchor="middle" fill="var(--mongodb)" fontFamily="var(--font-mono)" fontSize="12">
          MongoDB
        </text>
      </g>

      {/* Flow lines */}
      <path
        d="M222 210 H300"
        stroke="url(#flow-line)"
        strokeWidth="3"
        strokeLinecap="round"
        className="landing-draw"
      />
      <path
        d="M420 210 H498"
        stroke="url(#flow-line)"
        strokeWidth="3"
        strokeLinecap="round"
        className="landing-draw"
        style={{ animationDelay: '0.2s' }}
      />

      {/* Hub */}
      <g className="landing-pulse">
        <circle cx="360" cy="210" r="58" fill="var(--accent-soft)" />
        <circle cx="360" cy="210" r="42" fill="var(--accent)" />
        <path
          d="M344 210c0-8.8 7.2-16 16-16h4v8h-4a8 8 0 0 0 0 16h4v8h-4c-8.8 0-16-7.2-16-16Z"
          fill="white"
          opacity="0.4"
        />
        <path
          d="M376 210c0 8.8-7.2 16-16 16h-4v-8h4a8 8 0 0 0 0-16h-4v-8h4c8.8 0 16 7.2 16 16Z"
          fill="white"
        />
      </g>

      {/* Destination stack */}
      <g className="landing-float" style={{ animationDelay: '0.35s' }}>
        <rect x="498" y="110" width="150" height="200" rx="18" fill="var(--surface)" stroke="var(--border-strong)" />
        <text x="573" y="142" textAnchor="middle" fill="var(--text-muted)" fontFamily="var(--font-mono)" fontSize="11">
          DESTINATIONS
        </text>
        <rect x="518" y="162" width="110" height="28" rx="8" fill="var(--typesense-soft)" />
        <text x="573" y="181" textAnchor="middle" fill="var(--typesense)" fontFamily="var(--font-mono)" fontSize="12">
          Typesense
        </text>
        <rect x="518" y="202" width="110" height="28" rx="8" fill="var(--redis-soft)" />
        <text x="573" y="221" textAnchor="middle" fill="var(--redis)" fontFamily="var(--font-mono)" fontSize="12">
          Redis
        </text>
        <rect x="518" y="242" width="110" height="28" rx="8" fill="var(--sqlite-soft)" />
        <text x="573" y="261" textAnchor="middle" fill="var(--sqlite)" fontFamily="var(--font-mono)" fontSize="12">
          SQLite
        </text>
      </g>
    </svg>
  )
}

export default function LandingPage() {
  return (
    <div className="min-h-screen">
      <header className="animate-fade-in mx-auto flex max-w-6xl items-center justify-between px-5 py-6 sm:px-8">
        <div className="flex items-center gap-3">
          <PorticoMark />
          <span className="text-xl font-semibold tracking-tight text-[var(--text)]">Portico</span>
        </div>
        <Link
          to="/connections"
          className="rounded-lg border border-[var(--border)] bg-[var(--surface)] px-3.5 py-2 text-sm font-medium text-[var(--text)] shadow-[var(--shadow-sm)] transition-all hover:border-[var(--border-strong)] hover:bg-[var(--bg-elevated)]"
        >
          Open admin
        </Link>
      </header>

      <main className="mx-auto grid max-w-6xl items-center gap-10 px-5 pb-16 pt-6 sm:px-8 lg:grid-cols-[1fr_1.05fr] lg:gap-12 lg:pb-24 lg:pt-10">
        <div className="animate-fade-up max-w-xl">
          <p className="font-mono text-[11px] tracking-[0.22em] text-[var(--text-muted)] uppercase">
            Sync admin
          </p>
          <h1 className="mt-3 text-5xl font-semibold tracking-tight text-[var(--text)] sm:text-6xl">
            Portico
          </h1>
          <p className="mt-5 text-lg leading-relaxed text-[var(--text-muted)] sm:text-xl">
            Move data between MySQL, Postgres, SQLite, Typesense, MongoDB, and Redis — with
            fields, filters, and nested relations you control.
          </p>
          <div className="mt-8 flex flex-wrap gap-3">
            <Link
              to="/connections"
              className="inline-flex items-center justify-center rounded-lg bg-[var(--accent)] px-4 py-2.5 text-sm font-semibold text-white shadow-[var(--shadow-md)] transition-all hover:bg-[var(--accent-hover)] active:scale-[0.98]"
            >
              Open admin
            </Link>
            <Link
              to="/explore"
              className="inline-flex items-center justify-center rounded-lg border border-[var(--border)] bg-[var(--surface)] px-4 py-2.5 text-sm font-medium text-[var(--text)] shadow-[var(--shadow-sm)] transition-all hover:border-[var(--border-strong)] hover:bg-[var(--bg-elevated)] active:scale-[0.98]"
            >
              Explore data
            </Link>
          </div>
        </div>

        <div className="animate-fade-up stagger-2 flex justify-center lg:justify-end">
          <SyncScene />
        </div>
      </main>
    </div>
  )
}
