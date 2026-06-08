export default defineNuxtPlugin(() => {
  if (!import.meta.dev) return
  if (typeof navigator === 'undefined' || !('serviceWorker' in navigator)) return

  navigator.serviceWorker
    .getRegistrations()
    .then((regs) => {
      if (regs.length === 0) return

      Promise.all(regs.map((r) => r.unregister()))
        .then(() => ('caches' in window ? caches.keys() : Promise.resolve([])))
        .then((keys) => Promise.all(keys.map((k) => caches.delete(k))))
        .then(() => window.location.reload())
        .catch(() => {})
    })
    .catch(() => {})
})
