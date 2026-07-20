import { useState } from 'react'
import { Login } from './components/Login.jsx'
import { Dashboard } from './components/Dashboard.jsx'
import { Documents } from './components/Documents.jsx'
import { Conversations } from './components/Conversations.jsx'

const TABS = [
  ['overview', 'Overview'],
  ['documents', 'Knowledge'],
  ['conversations', 'Conversations'],
]

const TOKEN_KEY = 'docmind-admin-token'

function Workspace({ token, onSignOut }) {
  const [tab, setTab] = useState('overview')

  const panels = {
    overview: <Dashboard token={token} />,
    documents: <Documents token={token} />,
    conversations: <Conversations token={token} />,
  }

  return (
    <main className="app">
      <aside>
        <div className="brand">
          <p>DOCMIND</p>
          <strong>Admin workspace</strong>
        </div>
        {TABS.map(([id, label]) => (
          <button className={tab === id ? 'active' : ''} key={id} onClick={() => setTab(id)}>
            {label}
          </button>
        ))}
        <button className="logout" onClick={onSignOut}>Sign out</button>
      </aside>
      <section className="content">
        <header>
          <div>
            <p className="eyebrow">OPERATIONS</p>
            <h1>{tab[0].toUpperCase() + tab.slice(1)}</h1>
          </div>
          <span className="status">● Knowledge base online</span>
        </header>
        {panels[tab]}
      </section>
    </main>
  )
}

export function App() {
  const [token, setToken] = useState(() => localStorage.getItem(TOKEN_KEY) || '')

  function signIn(value) {
    localStorage.setItem(TOKEN_KEY, value)
    setToken(value)
  }

  function signOut() {
    localStorage.removeItem(TOKEN_KEY)
    setToken('')
  }

  if (!token) return <Login onLogin={signIn} />
  return <Workspace token={token} onSignOut={signOut} />
}
