import { NavLink, Outlet } from 'react-router-dom'

const navItems = [
  {
    to: '/connections',
    label: 'Connections',
    hint: 'Sources & destinations',
    icon: (
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true">
        <path
          d="M7 8a3 3 0 1 0 0-6 3 3 0 0 0 0 6ZM17 22a3 3 0 1 0 0-6 3 3 0 0 0 0 6ZM7.5 9.5 16 18.5M16.5 9.5 8 18.5"
          stroke="currentColor"
          strokeWidth="1.75"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </svg>
    ),
  },
  {
    to: '/sync-jobs',
    label: 'Sync Jobs',
    hint: 'Maps & runs',
    icon: (
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true">
        <path
          d="M4 7h12M4 12h16M4 17h9"
          stroke="currentColor"
          strokeWidth="1.75"
          strokeLinecap="round"
        />
        <path
          d="M17 5v4l3-2-3-2Z"
          fill="currentColor"
        />
      </svg>
    ),
  },
  {
    to: '/sync-logs',
    label: 'Sync Logs',
    hint: 'Run history',
    icon: (
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true">
        <path
          d="M5 4h14v16H5V4Z"
          stroke="currentColor"
          strokeWidth="1.75"
          strokeLinejoin="round"
        />
        <path d="M8 8h8M8 12h8M8 16h5" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" />
      </svg>
    ),
  },
]

export default function Layout() {
  return (
    <div className="min-h-screen lg:grid lg:grid-cols-[272px_1fr]">
      <aside className="relative border-b border-[var(--border)] bg-[var(--surface)]/90 backdrop-blur-md lg:border-b-0 lg:border-r">
        <div className="pointer-events-none absolute inset-x-0 top-0 h-32 bg-gradient-to-b from-[var(--accent-soft)]/70 to-transparent" />
        <div className="relative flex flex-col gap-8 px-5 py-6 lg:sticky lg:top-0 lg:min-h-screen lg:px-6 lg:py-8">
          <div className="animate-fade-up">
            <div className="flex items-center gap-3">
              <div className="flex h-11 w-11 items-center justify-center rounded-2xl bg-[var(--accent)] text-white shadow-[var(--shadow-md)]">
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
              <div>
                <p className="font-mono text-[10px] tracking-[0.22em] text-[var(--text-muted)] uppercase">
                  Sync admin
                </p>
                <h1 className="text-xl font-semibold tracking-tight text-[var(--text)]">Portico</h1>
              </div>
            </div>
          </div>

          <nav className="animate-fade-up stagger-1 flex gap-2 overflow-x-auto pb-1 lg:flex-col lg:overflow-visible lg:pb-0">
            {navItems.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                className={({ isActive }) =>
                  [
                    'group flex min-w-[148px] items-center gap-3 rounded-xl px-3 py-2.5 transition-all lg:min-w-0',
                    isActive
                      ? 'bg-[var(--accent)] text-white shadow-[var(--shadow-md)]'
                      : 'text-[var(--text-muted)] hover:bg-[var(--bg-elevated)] hover:text-[var(--text)]',
                  ].join(' ')
                }
              >
                {({ isActive }) => (
                  <>
                    <span
                      className={[
                        'flex h-9 w-9 items-center justify-center rounded-lg transition-colors',
                        isActive ? 'bg-white/15' : 'bg-[var(--bg-elevated)] group-hover:bg-white',
                      ].join(' ')}
                    >
                      {item.icon}
                    </span>
                    <span className="min-w-0">
                      <span className="block text-sm font-semibold">{item.label}</span>
                      <span
                        className={[
                          'hidden text-[11px] lg:block',
                          isActive ? 'text-white/75' : 'text-[var(--text-muted)]',
                        ].join(' ')}
                      >
                        {item.hint}
                      </span>
                    </span>
                  </>
                )}
              </NavLink>
            ))}
          </nav>
        </div>
      </aside>

      <main className="px-4 py-6 sm:px-8 sm:py-10">
        <div className="mx-auto max-w-6xl">
          <Outlet />
        </div>
      </main>
    </div>
  )
}
