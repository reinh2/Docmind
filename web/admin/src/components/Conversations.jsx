import { useEffect, useState } from 'react'
import { api, formatDate } from '../api.js'

function Message({ message }) {
  return (
    <article className={`message ${message.role}`}>
      <small>
        {message.role} · {formatDate(message.created_at)}
        {message.not_found ? ' · not found' : ''}
      </small>
      <p>{message.content}</p>
      {message.sources?.map(source => (
        <details key={source.chunk_id}>
          <summary>
            {source.document}{source.section && ` — ${source.section}`} <em>{source.score.toFixed(3)}</em>
          </summary>
          <pre>{source.excerpt}</pre>
        </details>
      ))}
    </article>
  )
}

export function Conversations({ token }) {
  const [items, setItems] = useState([])
  const [channel, setChannel] = useState('')
  const [selected, setSelected] = useState(null)
  const [error, setError] = useState('')

  useEffect(() => {
    const query = channel ? `?channel=${channel}` : ''
    api(token, `/v1/admin/conversations${query}`)
      .then(data => {
        setItems(data.conversations || [])
        setSelected(null)
      })
      .catch(e => setError(e.message))
  }, [token, channel])

  function open(id) {
    api(token, `/v1/admin/conversations/${id}`).then(setSelected).catch(e => setError(e.message))
  }

  return (
    <div className="split">
      <section className="panel conversation-list">
        <div className="panel-title">
          <h2>Conversations</h2>
          <select value={channel} onChange={event => setChannel(event.target.value)}>
            <option value="">All channels</option>
            <option value="widget">Widget</option>
            <option value="telegram">Telegram</option>
          </select>
        </div>
        {items.map(conversation => (
          <button className="conversation" key={conversation.id} onClick={() => open(conversation.id)}>
            <b>{conversation.channel}</b>
            <span>{conversation.message_count} messages · {formatDate(conversation.last_message_at)}</span>
          </button>
        ))}
        {!items.length && <p>No conversations yet.</p>}
      </section>
      <section className="panel transcript">
        <h2>Evidence inspector</h2>
        {error && <p className="error">{error}</p>}
        {!selected && <p>Select a conversation to see the complete transcript and chunks supplied to the model.</p>}
        {selected?.messages.map(message => (
          <Message key={message.id} message={message} />
        ))}
      </section>
    </div>
  )
}
