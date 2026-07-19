import { useEffect, useState } from 'react'
import { createRoot } from 'react-dom/client'
import './style.css'

const BASE = import.meta.env.VITE_API_BASE || '/api'
function api(token, path, options = {}) {
  const headers = new Headers(options.headers || {})
  headers.set('Authorization', `Bearer ${token}`)
  return fetch(`${BASE}${path}`, { ...options, headers }).then(async r => {
    if (!r.ok) throw new Error((await r.json().catch(() => ({}))).error || `Request failed (${r.status})`)
    return r.status === 204 ? null : r.json()
  })
}
const formatDate = value => new Date(value).toLocaleString()

function Login({ onLogin }) {
  const [token, setToken] = useState('demo-admin-token')
  return <main className="login"><section><p className="eyebrow">DOCMIND</p><h1>Knowledge you can audit.</h1><p>Enter the admin token to inspect documents, answer evidence, and customer signals.</p><form onSubmit={e => { e.preventDefault(); onLogin(token) }}><input value={token} onChange={e => setToken(e.target.value)} placeholder="Admin token" type="password" /><button>Open workspace</button></form></section></main>
}

function Stat({ label, value, note }) { return <article className="stat"><span>{label}</span><strong>{value}</strong>{note && <small>{note}</small>}</article> }
function Dashboard({ token }) {
  const [data, setData] = useState(null); const [error, setError] = useState('')
  useEffect(() => { api(token, '/v1/admin/dashboard').then(setData).catch(e => setError(e.message)) }, [token])
  if (error) return <p className="error">{error}</p>; if (!data) return <p>Loading dashboard…</p>
  return <><div className="stats"><Stat label="Questions today" value={data.questions_today} /><Stat label="Not found" value={`${Math.round(data.not_found_ratio * 100)}%`} /><Stat label="Escalations" value={`${Math.round(data.escalation_ratio * 100)}%`} /><Stat label="Average latency" value={`${Math.round(data.average_latency_ms)} ms`} /><Stat label="Feedback" value={`${data.positive_feedback} / ${data.negative_feedback}`} note="helpful / not helpful" /></div><section className="panel"><h2>Top customer questions</h2>{data.top_questions.length ? <ol className="questions">{data.top_questions.map(q => <li key={q.question}><span>{q.question}</span><b>{q.count}</b></li>)}</ol> : <p>No customer questions yet.</p>}</section></>
}

function Documents({ token }) {
  const [docs, setDocs] = useState([]); const [file, setFile] = useState(null); const [message, setMessage] = useState('')
  const load = () => api(token, '/v1/admin/documents').then(d => setDocs(d.documents || [])).catch(e => setMessage(e.message))
  useEffect(load, [token])
  async function upload(e) { e.preventDefault(); if (!file) return; const body = new FormData(); body.append('file', file); body.append('title', file.name); try { const r = await api(token, '/v1/admin/documents', { method: 'POST', body }); setMessage(r.duplicate ? 'This document is already indexed.' : `Ingestion job ${r.job.id} accepted.`); setFile(null); e.currentTarget.reset(); load() } catch (err) { setMessage(err.message) } }
  async function action(path, label) { try { const r = await api(token, path, { method: 'POST' }); setMessage(r?.job ? `${label}: ${r.job.id}` : label); load() } catch (e) { setMessage(e.message) } }
  async function remove(id) { if (!window.confirm('Remove this document from retrieval?')) return; try { await api(token, `/v1/admin/documents/${id}`, { method: 'DELETE' }); load() } catch (e) { setMessage(e.message) } }
  return <><section className="panel upload"><h2>Upload knowledge</h2><form onSubmit={upload}><input type="file" accept=".txt,.md,.markdown,.html,.htm,.pdf" onChange={e => setFile(e.target.files?.[0] || null)} /><button>Index document</button></form>{message && <p className="notice">{message}</p>}</section><section className="panel"><h2>Active documents</h2><div className="table">{docs.map(d => <div className="row" key={d.id}><div><b>{d.title}</b><small>{d.source_type} · {formatDate(d.created_at)}</small></div><div className="row-actions"><button onClick={() => action(`/v1/admin/documents/${d.id}/reindex`, 'Reindex queued')}>Reindex</button><button className="danger" onClick={() => remove(d.id)}>Remove</button></div></div>)}{!docs.length && <p>No documents indexed.</p>}</div></section></>
}

function Conversations({ token }) {
  const [items, setItems] = useState([]); const [channel, setChannel] = useState(''); const [selected, setSelected] = useState(null); const [error, setError] = useState('')
  useEffect(() => { api(token, `/v1/admin/conversations${channel ? `?channel=${channel}` : ''}`).then(d => { setItems(d.conversations || []); setSelected(null) }).catch(e => setError(e.message)) }, [token, channel])
  function open(id) { api(token, `/v1/admin/conversations/${id}`).then(setSelected).catch(e => setError(e.message)) }
  return <div className="split"><section className="panel conversation-list"><div className="panel-title"><h2>Conversations</h2><select value={channel} onChange={e => setChannel(e.target.value)}><option value="">All channels</option><option value="widget">Widget</option><option value="telegram">Telegram</option></select></div>{items.map(c => <button className="conversation" key={c.id} onClick={() => open(c.id)}><b>{c.channel}</b><span>{c.message_count} messages · {formatDate(c.last_message_at)}</span></button>)}{!items.length && <p>No conversations yet.</p>}</section><section className="panel transcript"><h2>Evidence inspector</h2>{error && <p className="error">{error}</p>}{!selected && <p>Select a conversation to see the complete transcript and chunks supplied to the model.</p>}{selected?.messages.map(m => <article className={`message ${m.role}`} key={m.id}><small>{m.role} · {formatDate(m.created_at)}{m.not_found ? ' · not found' : ''}</small><p>{m.content}</p>{m.sources?.map(s => <details key={s.chunk_id}><summary>{s.document}{s.section && ` — ${s.section}`} <em>{s.score.toFixed(3)}</em></summary><pre>{s.excerpt}</pre></details>)}</article>)}</section></div>
}

function App() { const [token, setToken] = useState(() => localStorage.getItem('docmind-admin-token') || ''); const [tab, setTab] = useState('overview'); if (!token) return <Login onLogin={value => { localStorage.setItem('docmind-admin-token', value); setToken(value) }} />; const content = tab === 'overview' ? <Dashboard token={token} /> : tab === 'documents' ? <Documents token={token} /> : <Conversations token={token} />; return <main className="app"><aside><div className="brand"><p>DOCMIND</p><strong>Admin workspace</strong></div>{[['overview','Overview'],['documents','Knowledge'],['conversations','Conversations']].map(([id,label]) => <button className={tab === id ? 'active' : ''} key={id} onClick={() => setTab(id)}>{label}</button>)}<button className="logout" onClick={() => { localStorage.removeItem('docmind-admin-token'); setToken('') }}>Sign out</button></aside><section className="content"><header><div><p className="eyebrow">OPERATIONS</p><h1>{tab[0].toUpperCase() + tab.slice(1)}</h1></div><span className="status">● Knowledge base online</span></header>{content}</section></main> }
createRoot(document.getElementById('root')).render(<App />)
