import { useState } from 'react'

export function Login({ onLogin }) {
	const [token, setToken] = useState('')

  function submit(event) {
    event.preventDefault()
    onLogin(token)
  }

  return (
    <main className="login">
      <section>
        <p className="eyebrow">DOCMIND</p>
        <h1>Knowledge you can audit.</h1>
        <p>Enter the admin token to inspect documents, answer evidence, and customer signals.</p>
        <form onSubmit={submit}>
          <input
            type="password"
            value={token}
            placeholder="Admin token"
            onChange={event => setToken(event.target.value)}
          />
          <button>Open workspace</button>
        </form>
      </section>
    </main>
  )
}
