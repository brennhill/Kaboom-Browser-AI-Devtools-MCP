// csp-safe-executor.ts — Pre-compiled executor for structured commands in MAIN world.

/**
 * CSP-Safe Structured Command Executor
 *
 * This function is injected into the page's MAIN world via:
 *   chrome.scripting.executeScript({ world: "MAIN", func: cspSafeExecutor, args: [command] })
 *
 * It MUST be fully self-contained — no closures over module-level variables.
 * Chrome serializes the function source at injection time. Any external reference
 * will be undefined at runtime.
 *
 * WHY THIS BYPASSES CSP:
 * Chrome's extension API injects the function natively (same mechanism as content
 * scripts declared in manifest.json). The page's CSP governs scripts loaded BY
 * the page — it has no authority over Chrome's extension injection pipeline.
 * The command argument is JSON data, not code. The executor resolves property
 * paths via bracket notation and calls methods via .apply() — standard JS
 * operations that CSP cannot restrict.
 *
 * IMPORTANT: this binding
 * DOM methods require correct `this` (e.g., document.querySelector needs
 * this === document). The executor tracks the parent object through the chain
 * and uses fn.apply(parent, args) for call steps.
 */

/* eslint-disable @typescript-eslint/no-explicit-any */

interface ExecutorStep {
  op: 'access' | 'index' | 'call' | 'construct'
  key?: string
  index?: number
  args?: ExecutorValue[]
}

interface ExecutorValue {
  type: 'literal' | 'undefined' | 'global' | 'chain' | 'array' | 'object'
  value?: string | number | boolean | null
  name?: string
  root?: ExecutorValue
  steps?: ExecutorStep[]
  elements?: ExecutorValue[]
  entries?: Array<{ key: string; value: ExecutorValue }>
}

interface ExecutorCommand {
  expr: ExecutorValue
  assign?: { target: ExecutorValue; steps: ExecutorStep[]; key: string }
}

export function cspSafeExecutor(command: ExecutorCommand): unknown {
  /* jscpd:ignore-start */
  // --- Inline serialize (self-contained, no external refs) ---
  function serialize(value: any, depth: number, seen: WeakSet<object>): any {
    // Track cycles and arrays: returns undefined when the value is neither.
    function serializeCycleOrArray(v: any, d: number, s: WeakSet<object>): any {
      if (s.has(v)) return '[Circular]'
      s.add(v)
      if (Array.isArray(v)) return v.slice(0, 100).map((item: any) => serialize(item, d + 1, s))
      return undefined
    }

    // Recognized non-plain objects. Wrapped so toJSON() returning undefined is preserved.
    function serializeSpecialValue(v: any, d: number, s: WeakSet<object>): { value: any } | null {
      if (v instanceof Error) return { value: { error: v.message } }
      if (v instanceof Date) return { value: v.toISOString() }
      if (v instanceof RegExp) return { value: String(v) }
      // DOM node duck-type check
      if ('nodeType' in v && 'nodeName' in v) {
        return { value: `[${v.nodeName}${v.id ? '#' + v.id : ''}]` }
      }
      // Browser host objects (DOMRect, DOMPoint, DOMMatrix) have prototype getters
      // that Object.keys() misses. Their toJSON() returns a plain object.
      if (typeof v.toJSON === 'function') {
        try {
          return { value: serialize(v.toJSON(), d + 1, s) }
        } catch {
          // EXPECTED_ABSENCE: optional enrichment can normally fail while the primary
          // operation keeps a valid fallback; logging it would misleadingly report fallback as failure.
          // Fall through to Object.keys() enumeration
        }
      }
      return null
    }

    function readHostPrimitive(v: any, key: string): { value: any } | null {
      try {
        const propValue = v[key]
        const valueType = typeof propValue
        if (propValue === undefined || valueType === 'function') return null
        if (valueType === 'string' || valueType === 'number' || valueType === 'boolean' || propValue === null) {
          return { value: propValue }
        }
        return null
      } catch {
        // EXPECTED_ABSENCE: optional enrichment can normally fail while the primary
        // operation keeps a valid fallback; logging it would misleadingly report fallback as failure.
        // Ignore getter access errors.
        return null
      }
    }

    // #389: Host objects may expose values only via prototype getters.
    // Capture primitive getter values when enumerable keys are absent.
    function serializeHostObject(v: any): Record<string, any> | undefined {
      try {
        const proto = Object.getPrototypeOf(v)
        if (proto && proto !== Object.prototype) {
          const hostResult: Record<string, any> = {}
          const propNames = Object.getOwnPropertyNames(proto).slice(0, 120)
          for (const key of propNames) {
            if (key === 'constructor') continue
            const primitive = readHostPrimitive(v, key)
            if (primitive) hostResult[key] = primitive.value
            if (Object.keys(hostResult).length >= 50) break
          }
          if (Object.keys(hostResult).length > 0) return hostResult
        }
      } catch {
        // EXPECTED_ABSENCE: optional enrichment can normally fail while the primary
        // operation keeps a valid fallback; logging it would misleadingly report fallback as failure.
        // Fall through to default object key enumeration.
      }
      return undefined
    }

    function serializePlainObject(v: any, keys: string[], d: number, s: WeakSet<object>): Record<string, any> {
      const result: Record<string, any> = {}
      for (const key of keys) {
        try {
          result[key] = serialize(v[key], d + 1, s)
        } catch {
          result[key] = '[unserializable]'
        }
      }
      return result
    }

    if (depth > 10) return '[max depth]'
    if (value === null || value === undefined) return value
    const t = typeof value
    if (t === 'string' || t === 'number' || t === 'boolean') return value
    if (t === 'function') return '[Function]'
    if (t === 'symbol') return String(value)
    if (t !== 'object') return String(value)
    const arrayOrCycle = serializeCycleOrArray(value, depth, seen)
    if (arrayOrCycle !== undefined) return arrayOrCycle
    const special = serializeSpecialValue(value, depth, seen)
    if (special) return special.value
    const keys = Object.keys(value).slice(0, 50)
    if (keys.length === 0) {
      const host = serializeHostObject(value)
      if (host) return host
    }
    return serializePlainObject(value, keys, depth, seen)
  }
  /* jscpd:ignore-end */

  // --- Resolve a StructuredValue to an actual JS value ---
  function resolveValue(val: ExecutorValue): any {
    switch (val.type) {
      case 'literal':
        return val.value
      case 'undefined':
        return undefined
      case 'global':
        return (globalThis as any)[val.name!]
      case 'array':
        return (val.elements || []).map((el: ExecutorValue) => resolveValue(el))
      case 'object': {
        const obj: Record<string, any> = {}
        for (const entry of val.entries || []) {
          obj[entry.key] = resolveValue(entry.value)
        }
        return obj
      }
      case 'chain':
        return resolveChain(val.root!, val.steps || [])
      default:
        throw new TypeError(`Unknown value type: ${(val as any).type}`)
    }
  }

  // --- Walk a chain of steps, preserving parent for this binding ---
  // parent tracks the object that owns the current value, so method calls
  // get the correct `this` (e.g., document.querySelector needs this === document).
  // Only access/index steps update parent; call/construct consume it.
  interface ChainState {
    parent: any
    current: any
  }

  function requireStepTarget(state: ChainState, describe: () => string): void {
    if (state.current === null || state.current === undefined) {
      throw new TypeError(describe())
    }
  }

  function applyStep(step: ExecutorStep, state: ChainState): ChainState {
    switch (step.op) {
      case 'access':
        requireStepTarget(state, () => `Cannot read property '${step.key}' of ${state.current}`)
        return { parent: state.current, current: state.current[step.key!] }
      case 'index':
        requireStepTarget(state, () => `Cannot read index ${step.index} of ${state.current}`)
        return { parent: state.current, current: state.current[step.index!] }
      case 'call': {
        if (typeof state.current !== 'function') {
          throw new TypeError(`${step.key || 'value'} is not a function`)
        }
        const callArgs = (step.args || []).map((a: ExecutorValue) => resolveValue(a))
        return { parent: null, current: state.current.apply(state.parent, callArgs) }
      }
      case 'construct': {
        if (typeof state.current !== 'function') {
          throw new TypeError(`${step.key || 'value'} is not a constructor`)
        }
        const constructArgs = (step.args || []).map((a: ExecutorValue) => resolveValue(a))
        return { parent: null, current: new state.current(...constructArgs) }
      }
      default:
        throw new TypeError(`Unknown step op: ${(step as any).op}`)
    }
  }

  function resolveChain(root: ExecutorValue, steps: ExecutorStep[]): any {
    let state: ChainState = { parent: null, current: resolveValue(root) }

    for (const step of steps) {
      state = applyStep(step, state)
    }
    return state.current
  }

  try {
    // Handle assignment
    if (command.assign) {
      const assignValue = resolveValue(command.expr)
      let target: any = resolveValue(command.assign.target)
      for (const step of command.assign.steps || []) {
        if (step.op === 'access') {
          target = target[step.key!]
        } else if (step.op === 'index') {
          target = target[step.index!]
        }
      }
      target[command.assign.key] = assignValue
      const result = serialize(assignValue, 0, new WeakSet())
      return { success: true, result, execution_mode: 'csp_safe_structured' }
    }

    // Normal expression evaluation
    const raw = resolveValue(command.expr)

    // Promise handling
    if (raw !== null && raw !== undefined && typeof raw.then === 'function') {
      return raw
        .then((v: unknown) => ({
          success: true,
          result: serialize(v, 0, new WeakSet()),
          execution_mode: 'csp_safe_structured'
        }))
        .catch((err: unknown) => ({
          success: false,
          error: 'promise_rejected',
          message: err instanceof Error ? err.message : String(err),
          execution_mode: 'csp_safe_structured'
        }))
    }

    return {
      success: true,
      result: serialize(raw, 0, new WeakSet()),
      execution_mode: 'csp_safe_structured'
    }
  } catch (err: unknown) {
    return {
      success: false,
      error: 'structured_execution_error',
      message: err instanceof Error ? err.message : String(err),
      execution_mode: 'csp_safe_structured'
    }
  }
}
