import { useEffect, useState } from 'react'

function randomToken() {
  return Math.random().toString(36).slice(2, 10)
}

export default function NextFixturePage() {
  const [token, setToken] = useState('next-bootstrap')
  const [mountToken, setMountToken] = useState('next-mount-bootstrap')
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
    setToken(randomToken())
    setMountToken(randomToken())
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

  globalThis.__SMOKE_FRAMEWORK__ = 'Next.js'
  globalThis.__SMOKE_FRAMEWORK_VERSION__ = 'runtime'
  globalThis.__SMOKE_SELECTOR_TOKEN__ = token
  globalThis.__SMOKE_ROUTE__ = route
  globalThis.__SMOKE_MOUNT_TOKEN__ = mountToken
  globalThis.__SMOKE_LOAD_ASYNC__ = loadAsyncPanel
  globalThis.__SMOKE_EXPAND_VIRTUAL__ = () => setVirtualExpanded(true)
  globalThis.__SMOKE_SHOW_PROFILE__ = () => switchRoute('profile')
  globalThis.__SMOKE_SHOW_SETTINGS__ = () => switchRoute('settings')

  return (
    <>
      <header className="kaboom-brand">
        <svg className="kaboom-brand-mark" viewBox="0 0 128 128" aria-hidden="true" focusable="false">
          <defs>
            <linearGradient id="nextBrandFlame" x1="0%" y1="100%" x2="0%" y2="0%">
              <stop offset="0%" stopColor="#f97316"></stop>
              <stop offset="55%" stopColor="#fb923c"></stop>
              <stop offset="100%" stopColor="#fbbf24"></stop>
            </linearGradient>
            <linearGradient id="nextBrandInnerFlame" x1="0%" y1="100%" x2="0%" y2="0%">
              <stop offset="0%" stopColor="#fbbf24"></stop>
              <stop offset="100%" stopColor="#fef3c7"></stop>
            </linearGradient>
          </defs>
          <circle cx="64" cy="64" r="60" fill="#121212"></circle>
          <path
            d="M64 16 C40 40, 28 60, 28 80 C28 100, 44 116, 64 116 C84 116, 100 100, 100 80 C100 60, 88 40, 64 16 Z"
            fill="url(#nextBrandFlame)"
          ></path>
          <path
            d="M64 48 C52 60, 44 72, 44 84 C44 96, 52 104, 64 104 C76 104, 84 96, 84 84 C84 72, 76 60, 64 48 Z"
            fill="url(#nextBrandInnerFlame)"
          ></path>
        </svg>
        <span>Kaboom Framework Smoke</span>
        <small>Next Selector Fixture</small>
      </header>

      <section className={`fixture-shell next-shell-${token}`}>
        <h1>Next Selector Fixture</h1>
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
      <script
        dangerouslySetInnerHTML={{
          __html: `(function () {
  if (window.__SMOKE_NEXT_FALLBACK_INIT__) return;
  window.__SMOKE_NEXT_FALLBACK_INIT__ = true;
  function randomToken() {
    return Math.random().toString(36).slice(2, 10);
  }
  function setMountToken() {
    var mount = randomToken();
    var input = document.querySelector('input[placeholder="Enter name"]');
    if (input) {
      input.id = 'name-' + mount;
      input.name = 'name-' + mount;
      input.value = '';
    }
    var buttons = Array.from(document.querySelectorAll('button'));
    var submit = buttons.find(function (b) {
      return (b.textContent || '').trim() === 'Submit Profile' && b.style.display !== 'none';
    });
    if (submit) {
      var replacement = submit.cloneNode(true);
      replacement.id = 'submit-' + mount;
      submit.replaceWith(replacement);
    }
    var mountNode = document.getElementById('mount-token');
    if (mountNode) mountNode.textContent = 'mount:' + mount;
    window.__SMOKE_MOUNT_TOKEN__ = mount;
  }
  var selectorNode = document.getElementById('selector-token');
  if (selectorNode) {
    var token = randomToken();
    selectorNode.textContent = 'token:' + token;
    window.__SMOKE_SELECTOR_TOKEN__ = token;
  }
  window.__SMOKE_FRAMEWORK__ = 'Next.js';
  window.__SMOKE_FRAMEWORK_VERSION__ = 'runtime-fallback';
  window.__SMOKE_ROUTE__ = 'profile';
  window.__SMOKE_SHOW_SETTINGS__ = function () {
    window.__SMOKE_ROUTE__ = 'settings';
    setMountToken();
  };
  window.__SMOKE_SHOW_PROFILE__ = function () {
    window.__SMOKE_ROUTE__ = 'profile';
    setMountToken();
  };
  window.__SMOKE_LOAD_ASYNC__ = function () {
    var resultNode = document.getElementById('async-result');
    if (resultNode) resultNode.textContent = 'loading';
    setTimeout(function () {
      if (resultNode) resultNode.textContent = 'ready';
      var panel = document.getElementById('async-panel');
      if (!panel) {
        panel = document.createElement('div');
        panel.id = 'async-panel';
        var button = document.createElement('button');
        button.type = 'button';
        button.textContent = 'Async Save';
        button.addEventListener('click', function () {
          if (resultNode) resultNode.textContent = 'async:clicked';
        });
        panel.appendChild(button);
        var anchor = document.getElementById('virtual-list');
        if (anchor && anchor.parentNode) {
          anchor.parentNode.insertBefore(panel, anchor);
        }
      }
    }, 600);
  };
  window.__SMOKE_EXPAND_VIRTUAL__ = function () {
    var list = document.getElementById('virtual-list');
    if (!list) return;
    if (!document.getElementById('deep-target')) {
      var button = document.createElement('button');
      button.id = 'deep-target';
      button.type = 'button';
      button.textContent = 'Framework Target 80';
      button.addEventListener('click', function () {
        var deepResultNode = document.getElementById('deep-result');
        if (deepResultNode) deepResultNode.textContent = 'deep:80';
      });
      list.appendChild(button);
    }
  };
  setTimeout(function () {
    var hydrating = document.getElementById('hydrating');
    if (hydrating) {
      hydrating.id = 'hydrated-ready';
      hydrating.textContent = 'hydrated';
    }
  }, 450);
})();`
        }}
      />
      <style jsx global>{`
        :root {
          --kaboom-ink: #17171d;
          --kaboom-muted: #4b5563;
          --kaboom-warm-100: #fff7ed;
          --kaboom-warm-200: #ffedd5;
          --kaboom-warm-500: #f97316;
          --kaboom-warm-600: #ea580c;
          --kaboom-border: #fdba74;
          --kaboom-shadow: rgba(234, 88, 12, 0.18);
        }
        body {
          margin: 0;
          font-family: 'Avenir Next', 'Inter', 'Segoe UI', sans-serif;
          background:
            radial-gradient(circle at top right, rgba(249, 115, 22, 0.12), transparent 38%),
            linear-gradient(140deg, var(--kaboom-warm-100), #fff);
          color: var(--kaboom-ink);
          min-height: 100vh;
          padding: 1.6rem 1.2rem 2.4rem;
        }
        .kaboom-brand {
          width: min(100%, 760px);
          margin: 0 auto 0.9rem;
          display: flex;
          align-items: center;
          gap: 0.65rem;
          color: #7c2d12;
          font-size: 0.9rem;
          font-weight: 700;
          letter-spacing: 0.02em;
        }
        .kaboom-brand-mark {
          width: 22px;
          height: 22px;
          flex: 0 0 22px;
        }
        .kaboom-brand small {
          color: var(--kaboom-muted);
          font-weight: 600;
        }
        .fixture-shell {
          line-height: 1.4;
          max-width: 760px;
          margin: 0 auto;
          padding: 1.25rem 1.25rem 1.3rem;
          border: 1px solid var(--kaboom-border);
          border-radius: 14px;
          background: #ffffff;
          box-shadow: 0 14px 32px -24px var(--kaboom-shadow);
        }
        .fixture-shell h1 {
          margin-top: 0;
          margin-bottom: 0.3rem;
          color: #9a3412;
          font-size: 1.44rem;
          letter-spacing: -0.01em;
        }
        .fixture-shell p {
          color: var(--kaboom-muted);
        }
        .fixture-shell label {
          display: block;
          font-weight: 600;
          margin-top: 0.75rem;
          color: #9a3412;
        }
        .fixture-shell input {
          width: 100%;
          max-width: 360px;
          padding: 0.5rem 0.6rem;
          border: 1px solid #fdba74;
          border-radius: 6px;
          background: #fffefc;
          color: var(--kaboom-ink);
        }
        .fixture-shell button {
          display: inline-block;
          margin-top: 0.8rem;
          padding: 0.55rem 1rem;
          border: none;
          border-radius: 6px;
          background: linear-gradient(180deg, var(--kaboom-warm-500), var(--kaboom-warm-600));
          color: #fff;
          cursor: pointer;
          font-weight: 700;
          box-shadow: 0 6px 16px -10px rgba(154, 52, 18, 0.75);
        }
        .fixture-nav {
          display: flex;
          gap: 0.5rem;
          margin-bottom: 0.75rem;
        }
        .fixture-nav button {
          margin-top: 0.15rem;
        }
        #virtual-list {
          margin-top: 1rem;
          max-width: 420px;
          height: 140px;
          overflow: auto;
          border: 1px solid #fdba74;
          border-radius: 8px;
          padding: 0.5rem;
          background: #fffbf6;
        }
        .virtual-row {
          font-size: 0.88rem;
          color: #6b7280;
          padding: 0.2rem 0;
        }
        #consent-modal {
          position: fixed;
          inset: 0;
          background: rgba(23, 23, 29, 0.55);
          z-index: 2147483600;
          display: flex;
          align-items: center;
          justify-content: center;
          flex-direction: column;
          color: #fff;
          gap: 0.6rem;
          padding: 1rem;
          text-align: center;
        }
        #consent-modal button {
          background: linear-gradient(180deg, var(--kaboom-warm-500), var(--kaboom-warm-600));
          margin-top: 0;
        }
      `}</style>
    </>
  )
}
