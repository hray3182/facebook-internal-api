chrome.runtime.onInstalled.addListener(() => {
  console.log('FBIA Cookie Exporter installed');
});

chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
  if (message.action === 'getCookies') {
    getCookies().then(sendResponse);
    return true;
  }
});

async function getCookies() {
  try {
    const cookies = await chrome.cookies.getAll({ domain: '.facebook.com' });
    const wwwCookies = await chrome.cookies.getAll({ domain: 'www.facebook.com' });
    const plainCookies = await chrome.cookies.getAll({ domain: 'facebook.com' });

    const allCookies = [...cookies, ...wwwCookies, ...plainCookies];
    const seen = new Map();

    for (const cookie of allCookies) {
      const key = `${cookie.name}@${cookie.domain}`;
      if (!seen.has(key)) {
        seen.set(key, {
          name: cookie.name,
          value: cookie.value,
          domain: cookie.domain,
          path: cookie.path,
          expires: cookie.expirationDate || 0,
          httpOnly: cookie.httpOnly || false,
          secure: cookie.secure || false,
          sameSite: cookie.sameSite || 'unspecified',
        });
      }
    }

    return { success: true, cookies: Array.from(seen.values()) };
  } catch (error) {
    return { success: false, error: error.message };
  }
}
