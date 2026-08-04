/**
 * Purpose: Surface redacted injected-world capture failures to local extension diagnostics.
 * Docs: docs/features/feature/system-doctor/index.md
 */

export interface PageCaptureDiagnostic {
  category: string
  message: string
  error_type: string
}

export function reportPageCaptureFailure(category: string, error: unknown): void {
  const diagnostic: PageCaptureDiagnostic = {
    category: category.slice(0, 64),
    message: 'Page capture subsystem failed; include System Doctor output in a bug report.',
    error_type: error instanceof Error ? error.name.slice(0, 64) : 'UnknownError'
  }
  console.error(`[KaBOOM!][${diagnostic.category}] ${diagnostic.message}`, { error_type: diagnostic.error_type })
  if (typeof window !== 'undefined') {
    try {
      window.postMessage({ type: 'kaboom_capture_diagnostic', payload: diagnostic }, window.location.origin)
    } catch (forwardError) {
      console.error('[KaBOOM!][page_capture] Failed to forward local Doctor diagnostic', {
        error_type: forwardError instanceof Error ? forwardError.name.slice(0, 64) : 'UnknownError'
      })
    }
  }
}
