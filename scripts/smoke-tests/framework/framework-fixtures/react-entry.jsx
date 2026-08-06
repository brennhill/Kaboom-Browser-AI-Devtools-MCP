import React, { useEffect, useMemo, useState } from 'react'
import { createRoot } from 'react-dom/client'

function randomToken() {
  return Math.random().toString(36).slice(2, 10)
}

function ReactFixtureApp() {
  const token = useMemo(() => randomToken(), [])
  const [mountToken, setMountToken] = useState(() => randomToken())
  const [route, setRoute] = useState('profile')
  const [hydrated, setHydrated] = useState(false)
  const [overlayOpen, setOverlayOpen] = useState(true)
  const [name, setName] = useState('')
  const [clicks, setClicks] = useState(0)
  const [result, setResult] = useState('idle')
  const [asyncReady, setAsyncReady] = useState(false)
  const [asyncResult, setAsyncResult] = useState('idle')
  const [virtualExpanded, setVirtualExpanded] = useState(false)
  const [deepResult, setDeepResult] = useState('idle')

  const inputId = `name-${mountToken}`
  const buttonId = `submit-${mountToken}`

  useEffect(() => {
    const handle = setTimeout(() => setHydrated(true), 450)
    return () => clearTimeout(handle)
  }, [])

  const submit = () => {
    if (!hydrated || overlayOpen) return
    const nextClicks = clicks + 1
    setClicks(nextClicks)
    setResult(`saved:${name || 'anonymous'}:${mountToken}:${nextClicks}`)
  }

  const switchRoute = (nextRoute) => {
    if (route === nextRoute) return
    setRoute(nextRoute)
    setMountToken(randomToken())
  }

  const loadAsyncPanel = () => {
    setAsyncReady(false)
    setAsyncResult('loading')
    setTimeout(() => {
      setAsyncReady(true)
      setAsyncResult('ready')
    }, 600)
  }

  const onVirtualScroll = (event) => {
    if (virtualExpanded) return
    const node = event.currentTarget
    if (node.scrollTop + node.clientHeight >= node.scrollHeight - 16) {
      setVirtualExpanded(true)
    }
  }

  globalThis.__SMOKE_FRAMEWORK__ = 'React'
  globalThis.__SMOKE_FRAMEWORK_VERSION__ = React.version
  globalThis.__SMOKE_SELECTOR_TOKEN__ = token
  globalThis.__SMOKE_ROUTE__ = route
  globalThis.__SMOKE_MOUNT_TOKEN__ = mountToken
  globalThis.__SMOKE_LOAD_ASYNC__ = loadAsyncPanel
  globalThis.__SMOKE_EXPAND_VIRTUAL__ = () => setVirtualExpanded(true)
  globalThis.__SMOKE_SHOW_PROFILE__ = () => switchRoute('profile')
  globalThis.__SMOKE_SHOW_SETTINGS__ = () => switchRoute('settings')

  return (
    <section className={`fixture-shell react-shell-${token}`}>
      <h1>React Selector Fixture</h1>
      <p>This page intentionally exercises hard automation cases.</p>
      <nav className="fixture-nav">
        <button type="button" onClick={() => switchRoute('profile')}>
          Profile Tab
        </button>
        <button type="button" onClick={() => switchRoute('settings')}>
          Settings Tab
        </button>
      </nav>
      {hydrated ? <p id="hydrated-ready">hydrated</p> : <p id="hydrating">hydrating</p>}

      {route === 'profile' ? (
        <div className="profile-card" key={mountToken}>
          <label htmlFor={inputId}>Name</label>
          <input
            id={inputId}
            name={inputId}
            placeholder="Enter name"
            value={name}
            onChange={(event) => setName(event.target.value)}
          />
          <button id={buttonId} type="button" onClick={submit} disabled={!hydrated || overlayOpen}>
            Submit Profile
          </button>
          <button type="button" style={{ display: 'none' }}>
            Submit Profile
          </button>
        </div>
      ) : (
        <div className="settings-card" key={mountToken}>
          <h2>Settings</h2>
          <p>Route remount churn fixture.</p>
        </div>
      )}

      <button type="button" onClick={loadAsyncPanel}>
        Load Async Panel
      </button>
      {asyncReady ? (
        <div id="async-panel">
          <button type="button" onClick={() => setAsyncResult('async:clicked')}>
            Async Save
          </button>
        </div>
      ) : null}

      <div id="virtual-list" onScroll={onVirtualScroll}>
        {Array.from({ length: virtualExpanded ? 100 : 24 }, (_, index) => {
          const row = index + 1
          if (row === 80) {
            return (
              <button type="button" id="deep-target" key={`row-${row}`} onClick={() => setDeepResult('deep:80')}>
                Framework Target 80
              </button>
            )
          }
          return (
            <div key={`row-${row}`} className="virtual-row">
              row:{row}
            </div>
          )
        })}
      </div>

      <p id="selector-token">token:{token}</p>
      <p id="mount-token">mount:{mountToken}</p>
      <p id="click-count">clicks:{clicks}</p>
      <p id="result">{result}</p>
      <p id="async-result">{asyncResult}</p>
      <p id="deep-result">{deepResult}</p>

      {overlayOpen ? (
        <div id="consent-modal" role="dialog" aria-modal="true">
          <p>Consent required before interacting with the form.</p>
          <button type="button" onClick={() => setOverlayOpen(false)}>
            Accept Cookies
          </button>
        </div>
      ) : null}
    </section>
  )
}

const mount = document.getElementById('react-root')
if (!mount) {
  throw new Error('missing #react-root mount node')
}

// Legacy detection hint used by the analyze(page_structure) smoke path.
mount.setAttribute('data-reactroot', '')
createRoot(mount).render(<ReactFixtureApp />)
