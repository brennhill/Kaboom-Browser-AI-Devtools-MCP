// sandbox-process.mjs — Process liveness and exit-wait helpers for the upgrade regression sandbox.

export function pidAlive(pid) {
  if (!pid || pid <= 0) return false
  try {
    process.kill(pid, 0)
    return true
  } catch {
    return false
  }
}

export async function waitForChildExit(child, timeoutMs = 10000) {
  if (!child) return true
  if (child.exitCode !== null || child.signalCode !== null) return true

  await new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      // On Windows, process handles can miss/lag exit events after external taskkill.
      // Treat "pid is already gone" as a successful exit to avoid false negatives.
      if (!pidAlive(child.pid)) {
        cleanup()
        resolve()
        return
      }
      cleanup()
      reject(new Error(`child ${child.pid} did not exit within ${timeoutMs}ms`))
    }, timeoutMs)

    const onExit = () => {
      cleanup()
      resolve()
    }
    const onError = (err) => {
      cleanup()
      reject(err)
    }
    const cleanup = () => {
      clearTimeout(timer)
      child.off('exit', onExit)
      child.off('error', onError)
    }

    child.on('exit', onExit)
    child.on('error', onError)
  })

  return true
}
