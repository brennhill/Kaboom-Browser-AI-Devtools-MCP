<script>
  function randomToken() {
    return Math.random().toString(36).slice(2, 10)
  }

  const token = randomToken()
  let mountToken = randomToken()
  let route = 'profile'
  let hydrated = false
  let overlayOpen = true
  let name = ''
  let clicks = 0
  let result = 'idle'
  let asyncReady = false
  let asyncResult = 'idle'
  let virtualExpanded = false
  let deepResult = 'idle'

  const hydrationHandle = setTimeout(() => {
    hydrated = true
  }, 450)

  function switchRoute(nextRoute) {
    if (route === nextRoute) return
    route = nextRoute
    mountToken = randomToken()
  }

  function submit() {
    if (!hydrated || overlayOpen) return
    clicks += 1
    result = `saved:${name || 'anonymous'}:${mountToken}:${clicks}`
  }

  function loadAsyncPanel() {
    asyncReady = false
    asyncResult = 'loading'
    setTimeout(() => {
      asyncReady = true
      asyncResult = 'ready'
    }, 600)
  }

  function onVirtualScroll(event) {
    if (virtualExpanded) return
    const node = event.currentTarget
    if (node.scrollTop + node.clientHeight >= node.scrollHeight - 16) {
      virtualExpanded = true
    }
  }

  function clickDeepTarget() {
    deepResult = 'deep:80'
  }

  function clickAsyncSave() {
    asyncResult = 'async:clicked'
  }

  function acceptCookies() {
    overlayOpen = false
  }

  $: inputId = `name-${mountToken}`
  $: buttonId = `submit-${mountToken}`
  $: rowCount = virtualExpanded ? 100 : 24
  $: rows = Array.from({ length: rowCount }, (_, index) => index + 1)

  globalThis.__SMOKE_FRAMEWORK__ = 'Svelte'
  globalThis.__SMOKE_FRAMEWORK_VERSION__ = 'runtime'
  globalThis.__SMOKE_SELECTOR_TOKEN__ = token
  globalThis.__SMOKE_LOAD_ASYNC__ = loadAsyncPanel
  globalThis.__SMOKE_EXPAND_VIRTUAL__ = () => {
    virtualExpanded = true
  }
  globalThis.__SMOKE_SHOW_PROFILE__ = () => switchRoute('profile')
  globalThis.__SMOKE_SHOW_SETTINGS__ = () => switchRoute('settings')
  $: globalThis.__SMOKE_ROUTE__ = route
  $: globalThis.__SMOKE_MOUNT_TOKEN__ = mountToken
</script>

<svelte:window on:beforeunload={() => clearTimeout(hydrationHandle)} />

<section class="fixture-shell svelte-shell-{token}">
  <h1>Svelte Selector Fixture</h1>
  <p>This page intentionally exercises hard automation cases.</p>
  <nav class="fixture-nav">
    <button type="button" on:click={() => switchRoute('profile')}>Profile Tab</button>
    <button type="button" on:click={() => switchRoute('settings')}>Settings Tab</button>
  </nav>
  {#if hydrated}
    <p id="hydrated-ready">hydrated</p>
  {:else}
    <p id="hydrating">hydrating</p>
  {/if}

  {#if route === 'profile'}
    <div class="profile-card">
      <label for={inputId}>Name</label>
      <input id={inputId} name={inputId} placeholder="Enter name" bind:value={name} />
      <button id={buttonId} type="button" on:click={submit} disabled={!hydrated || overlayOpen}>
        Submit Profile
      </button>
      <button type="button" style="display:none">Submit Profile</button>
    </div>
  {:else}
    <div class="settings-card">
      <h2>Settings</h2>
      <p>Route remount churn fixture.</p>
    </div>
  {/if}

  <button type="button" on:click={loadAsyncPanel}>Load Async Panel</button>
  {#if asyncReady}
    <div id="async-panel">
      <button type="button" on:click={clickAsyncSave}>Async Save</button>
    </div>
  {/if}

  <div id="virtual-list" on:scroll={onVirtualScroll}>
    {#each rows as row}
      {#if row === 80}
        <button id="deep-target" type="button" on:click={clickDeepTarget}>Framework Target 80</button>
      {:else}
        <div class="virtual-row">row:{row}</div>
      {/if}
    {/each}
  </div>

  <p id="selector-token">token:{token}</p>
  <p id="mount-token">mount:{mountToken}</p>
  <p id="click-count">clicks:{clicks}</p>
  <p id="result">{result}</p>
  <p id="async-result">{asyncResult}</p>
  <p id="deep-result">{deepResult}</p>

  {#if overlayOpen}
    <div id="consent-modal" role="dialog" aria-modal="true">
      <p>Consent required before interacting with the form.</p>
      <button type="button" on:click={acceptCookies}>Accept Cookies</button>
    </div>
  {/if}
</section>
