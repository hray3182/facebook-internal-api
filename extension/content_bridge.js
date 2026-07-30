// Isolated world: persist MAIN-world captures into chrome.storage.local.
window.addEventListener('message', (event) => {
  if (event.source !== window) {
    return;
  }
  const data = event.data;
  if (!data || data.source !== 'fbia-session-capture' || !data.session) {
    return;
  }
  if (!chrome.storage || !chrome.storage.local) {
    console.error('FBIA session bridge: chrome.storage.local unavailable');
    return;
  }
  chrome.storage.local.set(
    {
      session: data.session,
      session_friendly: data.friendly || '',
      session_captured_at: data.capturedAt || new Date().toISOString(),
    },
    () => {
      if (chrome.runtime.lastError) {
        console.error('FBIA session save failed:', chrome.runtime.lastError.message);
      }
    }
  );
});
