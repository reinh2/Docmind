import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, test } from 'vitest'
import { Login } from './Login.jsx'

afterEach(cleanup)

test('submits the token entered by the administrator', () => {
  let submittedToken = ''
  render(<Login onLogin={token => { submittedToken = token }} />)

  const input = screen.getByPlaceholderText('Admin token')
  fireEvent.change(input, { target: { value: 'local-secret' } })
  fireEvent.click(screen.getByRole('button', { name: 'Open workspace' }))

  expect(submittedToken).toBe('local-secret')
})

test('does not prefill a demonstration credential', () => {
  render(<Login onLogin={() => {}} />)
  expect(screen.getByPlaceholderText('Admin token')).toHaveValue('')
})
