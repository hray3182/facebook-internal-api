// Runs in the page MAIN world so we can observe Facebook's own fetch/XHR bodies.
(function () {
  const SESSION_KEYS = [
    '__aaid',
    '__ccg',
    '__crn',
    '__csr',
    '__dyn',
    '__hblp',
    '__hs',
    '__hsdp',
    '__hsi',
    '__req',
    '__rev',
    '__s',
    '__sjsp',
    '__spin_b',
    '__spin_r',
    '__spin_t',
    'dpr',
  ];

  function captureBody(body) {
    if (typeof body !== 'string' || body.indexOf('doc_id=') === -1) {
      return;
    }
    let params;
    try {
      params = new URLSearchParams(body);
    } catch (_) {
      return;
    }
    const session = {};
    for (const key of SESSION_KEYS) {
      if (params.has(key)) {
        const value = params.get(key);
        if (value) {
          session[key] = value;
        }
      }
    }
    // URLSearchParams.get decodes %3A → ":". GraphQL bodies carry the live
    // CSRF tokens; HTML DTSGInitialData is often stale and causes 1357004.
    const fb_dtsg = params.get('fb_dtsg') || '';
    const lsd = params.get('lsd') || '';
    if (Object.keys(session).length === 0 && !fb_dtsg && !lsd) {
      return;
    }
    window.postMessage(
      {
        source: 'fbia-session-capture',
        session,
        fb_dtsg,
        lsd,
        // Account that sent this request; the popup drops captures from other accounts.
        user: params.get('__user') || params.get('av') || '',
        friendly: params.get('fb_api_req_friendly_name') || '',
        capturedAt: new Date().toISOString(),
      },
      '*'
    );
  }

  function maybeCaptureRequest(input, init) {
    try {
      const url = typeof input === 'string' ? input : input && input.url;
      if (!url || String(url).indexOf('/api/graphql') === -1) {
        return;
      }
      const body = init && init.body;
      if (typeof body === 'string') {
        captureBody(body);
      } else if (body instanceof URLSearchParams) {
        captureBody(body.toString());
      }
    } catch (_) {
      // ignore
    }
  }

  const originalFetch = window.fetch;
  window.fetch = function (input, init) {
    maybeCaptureRequest(input, init);
    return originalFetch.apply(this, arguments);
  };

  const originalOpen = XMLHttpRequest.prototype.open;
  const originalSend = XMLHttpRequest.prototype.send;
  XMLHttpRequest.prototype.open = function (method, url) {
    this.__fbiaUrl = url;
    return originalOpen.apply(this, arguments);
  };
  XMLHttpRequest.prototype.send = function (body) {
    try {
      if (this.__fbiaUrl && String(this.__fbiaUrl).indexOf('/api/graphql') !== -1 && typeof body === 'string') {
        captureBody(body);
      }
    } catch (_) {
      // ignore
    }
    return originalSend.apply(this, arguments);
  };
})();
