import { createApp, h, ref, version } from 'vue'

function randomToken() {
  return Math.random().toString(36).slice(2, 10)
}

const token = randomToken()

const VueFixtureApp = {
  setup() {
    const mountToken = ref(randomToken())
    const route = ref('profile')
    const hydrated = ref(false)
    const overlayOpen = ref(true)
    const name = ref('')
    const clicks = ref(0)
    const result = ref('idle')
    const asyncReady = ref(false)
    const asyncResult = ref('idle')
    const virtualExpanded = ref(false)
    const deepResult = ref('idle')

    setTimeout(() => {
      hydrated.value = true
    }, 450)

    const submit = () => {
      if (!hydrated.value || overlayOpen.value) return
      clicks.value += 1
      result.value = `saved:${name.value || 'anonymous'}:${mountToken.value}:${clicks.value}`
    }

    const switchRoute = (nextRoute) => {
      if (route.value === nextRoute) return
      route.value = nextRoute
      mountToken.value = randomToken()
    }

    const loadAsyncPanel = () => {
      asyncReady.value = false
      asyncResult.value = 'loading'
      setTimeout(() => {
        asyncReady.value = true
        asyncResult.value = 'ready'
      }, 600)
    }

    const onVirtualScroll = (event) => {
      if (virtualExpanded.value) return
      const node = event.target
      if (node.scrollTop + node.clientHeight >= node.scrollHeight - 16) {
        virtualExpanded.value = true
      }
    }

    globalThis.__SMOKE_FRAMEWORK__ = 'Vue'
    globalThis.__SMOKE_FRAMEWORK_VERSION__ = version
    globalThis.__SMOKE_SELECTOR_TOKEN__ = token
    globalThis.__VUE__ = { version }
    globalThis.__SMOKE_LOAD_ASYNC__ = loadAsyncPanel
    globalThis.__SMOKE_EXPAND_VIRTUAL__ = () => {
      virtualExpanded.value = true
    }
    globalThis.__SMOKE_SHOW_PROFILE__ = () => switchRoute('profile')
    globalThis.__SMOKE_SHOW_SETTINGS__ = () => switchRoute('settings')

    return () => {
      const inputId = `name-${mountToken.value}`
      const buttonId = `submit-${mountToken.value}`

      globalThis.__SMOKE_ROUTE__ = route.value
      globalThis.__SMOKE_MOUNT_TOKEN__ = mountToken.value

      const profileNode =
        route.value === 'profile'
          ? h('div', { class: 'profile-card', key: mountToken.value }, [
              h('label', { for: inputId }, 'Name'),
              h('input', {
                id: inputId,
                name: inputId,
                value: name.value,
                placeholder: 'Enter name',
                onInput: (event) => {
                  name.value = event.target.value
                }
              }),
              h(
                'button',
                {
                  id: buttonId,
                  type: 'button',
                  disabled: !hydrated.value || overlayOpen.value,
                  onClick: submit
                },
                'Submit Profile'
              ),
              h(
                'button',
                {
                  type: 'button',
                  style: 'display:none'
                },
                'Submit Profile'
              )
            ])
          : h('div', { class: 'settings-card', key: mountToken.value }, [
              h('h2', 'Settings'),
              h('p', 'Route remount churn fixture.')
            ])

      const asyncPanel = asyncReady.value
        ? h('div', { id: 'async-panel' }, [
            h(
              'button',
              {
                type: 'button',
                onClick: () => {
                  asyncResult.value = 'async:clicked'
                }
              },
              'Async Save'
            )
          ])
        : null

      const virtualRows = Array.from({ length: virtualExpanded.value ? 100 : 24 }, (_, index) => {
        const row = index + 1
        if (row === 80) {
          return h(
            'button',
            {
              type: 'button',
              id: 'deep-target',
              key: `row-${row}`,
              onClick: () => {
                deepResult.value = 'deep:80'
              }
            },
            'Framework Target 80'
          )
        }
        return h(
          'div',
          {
            class: 'virtual-row',
            key: `row-${row}`
          },
          `row:${row}`
        )
      })

      const overlay = overlayOpen.value
        ? h(
            'div',
            {
              id: 'consent-modal',
              role: 'dialog',
              'aria-modal': 'true'
            },
            [
              h('p', 'Consent required before interacting with the form.'),
              h(
                'button',
                {
                  type: 'button',
                  onClick: () => {
                    overlayOpen.value = false
                  }
                },
                'Accept Cookies'
              )
            ]
          )
        : null

      return h('section', { class: `fixture-shell vue-shell-${token}` }, [
        h('h1', 'Vue Selector Fixture'),
        h('p', 'This page intentionally exercises hard automation cases.'),
        h('nav', { class: 'fixture-nav' }, [
          h(
            'button',
            {
              type: 'button',
              onClick: () => switchRoute('profile')
            },
            'Profile Tab'
          ),
          h(
            'button',
            {
              type: 'button',
              onClick: () => switchRoute('settings')
            },
            'Settings Tab'
          )
        ]),
        hydrated.value ? h('p', { id: 'hydrated-ready' }, 'hydrated') : h('p', { id: 'hydrating' }, 'hydrating'),
        profileNode,
        h(
          'button',
          {
            type: 'button',
            onClick: loadAsyncPanel
          },
          'Load Async Panel'
        ),
        asyncPanel,
        h(
          'div',
          {
            id: 'virtual-list',
            onScroll: onVirtualScroll
          },
          virtualRows
        ),
        h('p', { id: 'selector-token' }, `token:${token}`),
        h('p', { id: 'mount-token' }, `mount:${mountToken.value}`),
        h('p', { id: 'click-count' }, `clicks:${clicks.value}`),
        h('p', { id: 'result' }, result.value),
        h('p', { id: 'async-result' }, asyncResult.value),
        h('p', { id: 'deep-result' }, deepResult.value),
        overlay
      ])
    }
  }
}

createApp(VueFixtureApp).mount('#vue-root')
