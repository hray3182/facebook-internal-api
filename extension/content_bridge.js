// Isolated world: persist MAIN-world captures into chrome.storage.local.
window.addEventListener('message', (event) => {
  if (event.source !== window) {
    return;
  }
  const data = event.data;
  if (!data || data.source !== 'fbia-session-capture') {
    return;
  }
  const hasSession = data.session && typeof data.session === 'object' && Object.keys(data.session).length > 0;
  const fbDtsg = typeof data.fb_dtsg === 'string' ? data.fb_dtsg : '';
  const lsd = typeof data.lsd === 'string' ? data.lsd : '';
  if (!hasSession && !fbDtsg && !lsd) {
    return;
  }
  if (!chrome.storage || !chrome.storage.local) {
    console.error('FBIA session bridge: chrome.storage.local unavailable');
    return;
  }

  const capturedAt = data.capturedAt || new Date().toISOString();
  const user = typeof data.user === 'string' ? data.user : '';
  const update = {};
  if (hasSession) {
    update.session = data.session;
    update.session_user = user;
    update.session_friendly = data.friendly || '';
    update.session_captured_at = capturedAt;
  }
  // Each token records its own owner: a capture may carry only one of them.
  if (fbDtsg) {
    update.fb_dtsg = fbDtsg;
    update.fb_dtsg_user = user;
    update.tokens_captured_at = capturedAt;
    update.tokens_friendly = data.friendly || '';
  }
  if (lsd) {
    update.lsd = lsd;
    update.lsd_user = user;
    if (!update.tokens_captured_at) {
      update.tokens_captured_at = capturedAt;
    }
  }
  chrome.storage.local.set(update, () => {
    if (chrome.runtime.lastError) {
      console.error('FBIA session save failed:', chrome.runtime.lastError.message);
    }
  });
});
