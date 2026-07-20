import { useCallback, useEffect, useState } from 'react'
import { api, formatDate } from '../api.js'

export function Documents({ token }) {
  const [documents, setDocuments] = useState([])
  const [file, setFile] = useState(null)
  const [message, setMessage] = useState('')

  const load = useCallback(() => {
    api(token, '/v1/admin/documents')
      .then(data => setDocuments(data.documents || []))
      .catch(e => setMessage(e.message))
  }, [token])

  useEffect(() => { void load() }, [load])

  async function upload(event) {
    event.preventDefault()
    if (!file) return
    const body = new FormData()
    body.append('file', file)
    body.append('title', file.name)
    try {
      const result = await api(token, '/v1/admin/documents', { method: 'POST', body })
      setMessage(result.duplicate ? 'This document is already indexed.' : `Ingestion job ${result.job.id} accepted.`)
      setFile(null)
      event.target.reset()
      load()
    } catch (err) {
      setMessage(err.message)
    }
  }

  async function reindex(id) {
    try {
      const result = await api(token, `/v1/admin/documents/${id}/reindex`, { method: 'POST' })
      setMessage(result?.job ? `Reindex queued: ${result.job.id}` : 'Reindex queued')
      load()
    } catch (err) {
      setMessage(err.message)
    }
  }

  async function remove(id) {
    if (!window.confirm('Remove this document from retrieval?')) return
    try {
      await api(token, `/v1/admin/documents/${id}`, { method: 'DELETE' })
      load()
    } catch (err) {
      setMessage(err.message)
    }
  }

  return (
    <>
      <section className="panel upload">
        <h2>Upload knowledge</h2>
        <form onSubmit={upload}>
          <input
            type="file"
            accept=".txt,.md,.markdown,.html,.htm,.pdf"
            onChange={event => setFile(event.target.files?.[0] || null)}
          />
          <button>Index document</button>
        </form>
        {message && <p className="notice">{message}</p>}
      </section>
      <section className="panel">
        <h2>Active documents</h2>
        <div className="table">
          {documents.map(document => (
            <div className="row" key={document.id}>
              <div>
                <b>{document.title}</b>
                <small>{document.source_type} · {formatDate(document.created_at)}</small>
              </div>
              <div className="row-actions">
                <button onClick={() => reindex(document.id)}>Reindex</button>
                <button className="danger" onClick={() => remove(document.id)}>Remove</button>
              </div>
            </div>
          ))}
          {!documents.length && <p>No documents indexed.</p>}
        </div>
      </section>
    </>
  )
}
