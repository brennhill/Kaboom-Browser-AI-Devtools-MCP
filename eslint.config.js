import js from '@eslint/js'
import security from 'eslint-plugin-security'
import globals from 'globals'
import prettier from 'eslint-config-prettier'

export default [
  // Ignore patterns
  {
    ignores: [
      'node_modules/',
      'dist/',
      'coverage/',
      'tests/e2e/',
      'npm/',
      'server/',
      'docs/',
      'demo/',
      '.venv-build/',
      '.gasoline/',
      '.worktrees/',
      '.claude/',
      '.codex/',
      'gokaboom.dev/dist/',
      'pypi/**/build/',
      'pypi/**/dist/',
      'pypi/**/*.egg-info/',
      // Generated framework smoke artifacts
      'cmd/browser-agent/internal/testpages/pages/frameworks/**',
      'cmd/browser-agent/internal/testpages/pages/_next/**',
      'scripts/smoke-tests/framework/framework-fixtures/next-app/.next/**',
      'scripts/smoke-tests/framework/framework-fixtures/next-app/out/**',
      // Compiled TypeScript output (linted at the .ts source level)
      'extension/background/',
      'extension/content/',
      'extension/inject/',
      'extension/lib/',
      'extension/popup/',
      'extension/types/',
      'extension/offscreen/',
      // Bundled files
      'extension/content.bundled.js',
      'extension/inject.bundled.js',
      'extension/early-patch.bundled.js',
      'extension/offscreen.bundled.js',
      'extension/popup.bundled.js',
      // Vendored terminal assets (xterm.js) — both the legacy path and the
      // refactored internal/terminal location.
      'cmd/browser-agent/terminal_assets/**',
      'cmd/browser-agent/internal/terminal/assets/terminal_assets/**',
      // Generated smoke fixture bundles
      'cmd/browser-agent/internal/testpages/pages/frameworks/*.bundle.js'
    ]
  },

  // Base recommended rules
  js.configs.recommended,

  // Extension source files (run in browser / service worker)
  {
    files: ['extension/**/*.js'],
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: 'module',
      globals: {
        ...globals.browser,
        chrome: 'readonly',
        clients: 'readonly',
        registration: 'readonly',
        self: 'readonly',
        __KABOOM_VERSION__: 'readonly'
      }
    },
    plugins: {
      security
    },
    rules: {
      'no-var': 'error',
      'prefer-const': ['error', { destructuring: 'all' }],
      eqeqeq: ['error', 'always', { null: 'ignore' }],
      'prefer-arrow-callback': 'error',
      'no-unused-vars': [
        'error',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_', caughtErrorsIgnorePattern: '^_' }
      ],

      // Security rules
      'security/detect-object-injection': 'warn',
      'security/detect-non-literal-regexp': 'warn',
      'security/detect-eval-with-expression': 'error',
      // eslint-plugin-security rule is incompatible with ESLint 10 context API
      'security/detect-no-csrf-before-method-override': 'off',
      'security/detect-possible-timing-attacks': 'warn',

      // Best practices
      'no-eval': 'error',
      'no-implied-eval': 'error',
      'no-new-func': 'error',
      'no-script-url': 'error',
      'no-proto': 'error',
      'no-extend-native': 'error',
      'no-throw-literal': 'error',
      'no-promise-executor-return': 'error',
      'no-constructor-return': 'error',
      'no-template-curly-in-string': 'warn',
      'no-loss-of-precision': 'error',
      'require-atomic-updates': 'error',
      // Fail loud (CLAUDE.md rule 25): a truly-empty `catch {}` silently swallows
      // errors on a mutating path. Explicit here (not just via js:recommended) so
      // the intent survives config changes. A catch that must swallow documents
      // why with a comment; comment-only catches still pass no-empty by design.
      'no-empty': ['error', { allowEmptyCatch: false }]
    }
  },

  // Extension test files (run in Node.js)
  {
    files: ['tests/extension/**/*.js', 'tests/extension/**/*.mjs', 'tests/extension/**/*.cjs'],
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: 'module',
      globals: {
        ...globals.node,
        globalThis: 'readonly',
        document: 'readonly',
        chrome: 'readonly'
      }
    },
    plugins: {
      security
    },
    rules: {
      'no-var': 'error',
      'prefer-const': ['error', { destructuring: 'all' }],
      eqeqeq: ['error', 'always', { null: 'ignore' }],
      'no-unused-vars': [
        'error',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_', caughtErrorsIgnorePattern: '^_' }
      ],

      // Relaxed security for tests
      'security/detect-object-injection': 'off',
      'security/detect-non-literal-regexp': 'off',
      'security/detect-possible-timing-attacks': 'off',

      // Best practices
      'no-eval': 'error',
      'no-implied-eval': 'error',
      'no-new-func': 'error'
    }
  },

  // CLI test files (CommonJS, run in Node.js)
  {
    files: ['tests/cli/**/*.cjs'],
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: 'commonjs',
      globals: {
        ...globals.node
      }
    },
    rules: {
      'no-unused-vars': ['error', { argsIgnorePattern: '^_', varsIgnorePattern: '^_', caughtErrorsIgnorePattern: '^_' }]
    }
  },

  // Scripts (CommonJS, run in Node.js). The repo is "type": "module", so a Node
  // script that needs require/__dirname must be .cjs and linted as commonjs.
  {
    files: ['scripts/**/*.cjs'],
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: 'commonjs',
      globals: {
        ...globals.node
      }
    },
    plugins: {
      security
    }
  },

  // Scripts (ESM, run in Node.js)
  {
    files: ['scripts/**/*.js', 'scripts/**/*.mjs'],
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: 'module',
      globals: {
        ...globals.node
      }
    },
    plugins: {
      security
    },
    rules: {
      'no-unused-vars': ['error', { argsIgnorePattern: '^_', varsIgnorePattern: '^_', caughtErrorsIgnorePattern: '^_' }]
    }
  },

  // Kaboom CI package (runs in browser)
  {
    files: ['packages/kaboom-ci/**/*.js'],
    languageOptions: {
      ecmaVersion: 2020,
      sourceType: 'script',
      globals: {
        ...globals.browser
      }
    },
    plugins: {
      security
    },
    rules: {
      'no-var': 'off', // Legacy code uses var
      'prefer-const': 'off',
      'prefer-arrow-callback': 'off',
      'no-unused-vars': ['error', { argsIgnorePattern: '^_', varsIgnorePattern: '^_', caughtErrorsIgnorePattern: '^_' }]
    }
  },

  // Kaboom Playwright package (CommonJS, run in Node.js)
  {
    files: ['packages/kaboom-playwright/**/*.js'],
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: 'commonjs',
      globals: {
        ...globals.node
      }
    },
    rules: {
      'no-unused-vars': ['error', { argsIgnorePattern: '^_', varsIgnorePattern: '^_', caughtErrorsIgnorePattern: '^_' }]
    }
  },

  // Daemon-embedded page scripts (evaluated in the page's MAIN world). Each file
  // is a single expression the injected executor compiles with `return (<script>)`,
  // so it is a script, not a module.
  {
    files: ['cmd/browser-agent/internal/toolinteract/pagescripts/**/*.js'],
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: 'script',
      globals: {
        ...globals.browser
      }
    },
    plugins: {
      security
    },
    rules: {
      'no-empty': ['error', { allowEmptyCatch: false }]
    }
  },

  // Website demo/public snippets (run in browser)
  {
    files: ['gokaboom.dev/public/**/*.js'],
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: 'script',
      globals: {
        ...globals.browser
      }
    }
  },

  // Website utility scripts (run in Node.js)
  {
    files: ['gokaboom.dev/scripts/**/*.js', 'gokaboom.dev/scripts/**/*.mjs'],
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: 'module',
      globals: {
        ...globals.node
      }
    }
  },

  // Prettier must be last to disable conflicting format rules
  prettier
]
