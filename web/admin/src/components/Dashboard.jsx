import { useEffect, useState } from 'react'
import { api } from '../api.js'

function Stat({ label, value, note }) {
  return (
    <article className="stat">
      <span>{label}</span>
      <strong>{value}</strong>
      {note && <small>{note}</small>}
    </article>
  )
}

export function Dashboard({ token }) {
  const [data, setData] = useState(null)
  const [error, setError] = useState('')

  useEffect(() => {
    api(token, '/v1/admin/dashboard').then(setData).catch(e => setError(e.message))
  }, [token])

  if (error) return <p className="error">{error}</p>
  if (!data) return <p>Loading dashboard…</p>

  const percent = ratio => `${Math.round(ratio * 100)}%`

  return (
    <>
      <div className="stats">
        <Stat label="Questions today" value={data.questions_today} />
        <Stat label="Not found" value={percent(data.not_found_ratio)} />
        <Stat label="Escalations" value={percent(data.escalation_ratio)} />
        <Stat label="Average latency" value={`${Math.round(data.average_latency_ms)} ms`} />
        <Stat
          label="Feedback"
          value={`${data.positive_feedback} / ${data.negative_feedback}`}
          note="helpful / not helpful"
        />
      </div>
      <section className="panel">
        <h2>Top customer questions</h2>
        {data.top_questions.length ? (
          <ol className="questions">
            {data.top_questions.map(question => (
              <li key={question.question}>
                <span>{question.question}</span>
                <b>{question.count}</b>
              </li>
            ))}
          </ol>
        ) : (
          <p>No customer questions yet.</p>
        )}
      </section>
    </>
  )
}
