const loginStatus = document.getElementById('login-status');
const userName = document.getElementById('user-name');
const userId = document.getElementById('user-id');
const cookieCount = document.getElementById('cookie-count');
const dtsgStatus = document.getElementById('dtsg-status');
const lsdStatus = document.getElementById('lsd-status');
const downloadBtn = document.getElementById('download-btn');
const downloadAuthBtn = document.getElementById('download-auth-btn');
const resultDiv = document.getElementById('result');

let currentCookies = [];
let currentUserId = null;
let currentFbDtsg = null;
let currentLsd = null;

document.addEventListener('DOMContentLoaded', () => {
  checkLoginStatus();
});

downloadBtn.addEventListener('click', downloadCookies);
downloadAuthBtn.addEventListener('click', downloadAuth);

function updateAuthDownloadState() {
  downloadAuthBtn.disabled = !(currentFbDtsg && currentCookies.length > 0);
}

async function checkLoginStatus() {
  loginStatus.textContent = '檢查中...';
  loginStatus.className = 'status-badge checking';
  userName.textContent = '-';
  userId.textContent = '-';
  dtsgStatus.textContent = '-';
  dtsgStatus.className = 'status-badge checking';
  lsdStatus.textContent = '-';
  lsdStatus.className = 'status-badge checking';
  currentFbDtsg = null;
  currentLsd = null;
  updateAuthDownloadState();

  try {
    const cookies = await chrome.cookies.getAll({ domain: '.facebook.com' });
    const wwwCookies = await chrome.cookies.getAll({ domain: 'www.facebook.com' });
    const plainCookies = await chrome.cookies.getAll({ domain: 'facebook.com' });

    const uniqueCookies = deduplicateCookies([...cookies, ...wwwCookies, ...plainCookies]);
    currentCookies = uniqueCookies.map((c) => ({
      name: c.name,
      value: c.value,
      domain: c.domain,
      path: c.path,
      expires: c.expirationDate || 0,
      httpOnly: c.httpOnly || false,
      secure: c.secure || false,
      sameSite: c.sameSite || 'unspecified',
    }));

    const userCookie = currentCookies.find((c) => c.name === 'c_user');
    currentUserId = userCookie ? userCookie.value : null;
    cookieCount.textContent = currentCookies.length;

    if (currentUserId && currentCookies.length > 0) {
      loginStatus.textContent = '已登入';
      loginStatus.className = 'status-badge logged-in';
      userId.textContent = currentUserId;
      downloadBtn.disabled = false;
      await fetchUserName();
      await extractAuthTokens();
    } else {
      loginStatus.textContent = '未登入';
      loginStatus.className = 'status-badge not-logged-in';
      downloadBtn.disabled = true;
      showResult('請先在瀏覽器中登入 Facebook', 'info');
    }
  } catch (error) {
    loginStatus.textContent = '錯誤';
    loginStatus.className = 'status-badge not-logged-in';
    showResult('檢查登入狀態時發生錯誤: ' + error.message, 'error');
  }
}

async function fetchUserName() {
  userName.textContent = '讀取中...';
  try {
    const tabs = await chrome.tabs.query({ url: '*://*.facebook.com/*' });
    if (tabs.length === 0) {
      userName.textContent = '(請開啟 Facebook 頁面)';
      return;
    }

    const results = await chrome.scripting.executeScript({
      target: { tabId: tabs[0].id },
      func: extractUserInfo,
    });

    if (results && results[0] && results[0].result && results[0].result.name) {
      userName.textContent = results[0].result.name;
    } else {
      userName.textContent = '(無法取得)';
    }
  } catch (error) {
    console.error('Failed to fetch user name:', error);
    userName.textContent = '(無法取得)';
  }
}

async function extractAuthTokens() {
  dtsgStatus.textContent = '提取中...';
  dtsgStatus.className = 'status-badge checking';
  lsdStatus.textContent = '提取中...';
  lsdStatus.className = 'status-badge checking';

  try {
    const tabs = await chrome.tabs.query({ url: '*://*.facebook.com/*' });
    if (tabs.length === 0) {
      dtsgStatus.textContent = '(請開啟 Facebook)';
      dtsgStatus.className = 'status-badge not-logged-in';
      lsdStatus.textContent = '(請開啟 Facebook)';
      lsdStatus.className = 'status-badge not-logged-in';
      return;
    }

    const results = await chrome.scripting.executeScript({
      target: { tabId: tabs[0].id },
      func: extractTokensFromPage,
    });

    const tokens = results && results[0] && results[0].result ? results[0].result : null;
    currentFbDtsg = tokens && tokens.fb_dtsg ? tokens.fb_dtsg : null;
    currentLsd = tokens && tokens.lsd ? tokens.lsd : null;

    if (currentFbDtsg) {
      dtsgStatus.textContent = currentFbDtsg.substring(0, 16) + '...';
      dtsgStatus.className = 'status-badge logged-in';
    } else {
      dtsgStatus.textContent = '提取失敗';
      dtsgStatus.className = 'status-badge not-logged-in';
    }

    if (currentLsd) {
      lsdStatus.textContent = currentLsd.substring(0, 16) + '...';
      lsdStatus.className = 'status-badge logged-in';
    } else {
      lsdStatus.textContent = '提取失敗';
      lsdStatus.className = 'status-badge not-logged-in';
    }

    updateAuthDownloadState();
  } catch (error) {
    console.error('Failed to extract auth tokens:', error);
    dtsgStatus.textContent = '提取失敗';
    dtsgStatus.className = 'status-badge not-logged-in';
    lsdStatus.textContent = '提取失敗';
    lsdStatus.className = 'status-badge not-logged-in';
    updateAuthDownloadState();
  }
}

function extractTokensFromPage() {
  try {
    const html = document.documentElement.innerHTML;

    const dtsgPatterns = [
      /\["DTSGInitData",\[\],\{"token":"([^"]+)"/,
      /"DTSGInitialData".*?"token":"([^"]+)"/,
      /"dtsg":\{"token":"([^"]+)"/,
    ];
    let fb_dtsg = null;
    for (const pat of dtsgPatterns) {
      const m = html.match(pat);
      if (m) {
        fb_dtsg = m[1];
        break;
      }
    }

    const lsdPatterns = [
      /\["LSD",\[\],\{"token":"([^"]+)"\}/,
      /"LSD",\[\],\{"token":"([^"]+)"\}/,
      /name="lsd"\s+value="([^"]+)"/,
      /"lsd"\s*:\s*"([^"]+)"/,
    ];
    let lsd = null;
    for (const pat of lsdPatterns) {
      const m = html.match(pat);
      if (m) {
        lsd = m[1];
        break;
      }
    }

    return { fb_dtsg, lsd };
  } catch (_) {
    return { fb_dtsg: null, lsd: null };
  }
}

function extractUserInfo() {
  try {
    const profileLink = document.querySelector('a[href*="/me/"]');
    if (profileLink) {
      const name = profileLink.textContent?.trim();
      if (name) return { name };
    }

    const accountMenu = document.querySelector(
      '[aria-label="帳號"] span, [aria-label="Account"] span, [aria-label="你的個人檔案"] span'
    );
    if (accountMenu) {
      const name = accountMenu.textContent?.trim();
      if (name) return { name };
    }

    return { name: null };
  } catch (_) {
    return { name: null };
  }
}

function deduplicateCookies(cookies) {
  const seen = new Map();
  for (const cookie of cookies) {
    const key = `${cookie.name}@${cookie.domain}`;
    if (!seen.has(key)) {
      seen.set(key, cookie);
    }
  }
  return Array.from(seen.values());
}

async function downloadAuth() {
  if (!currentFbDtsg || currentCookies.length === 0) {
    showResult('缺少 cookies 或 fb_dtsg（請開啟 Facebook 分頁後重試）', 'error');
    return;
  }

  downloadAuthBtn.classList.add('loading');
  downloadAuthBtn.disabled = true;

  try {
    const authData = {
      cookies: currentCookies,
      fb_dtsg: currentFbDtsg,
      lsd: currentLsd,
      c_user: currentUserId,
      extracted_at: new Date().toISOString(),
    };

    const blob = new Blob([JSON.stringify(authData, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    await chrome.downloads.download({
      url,
      filename: 'auth.json',
      saveAs: true,
    });
    showResult('已下載 auth.json（含 cookies + fb_dtsg + lsd）', 'success');
  } catch (error) {
    showResult('下載失敗: ' + error.message, 'error');
  } finally {
    downloadAuthBtn.classList.remove('loading');
    updateAuthDownloadState();
  }
}

async function downloadCookies() {
  if (currentCookies.length === 0) {
    showResult('沒有可下載的 cookies', 'error');
    return;
  }

  downloadBtn.classList.add('loading');
  downloadBtn.disabled = true;

  try {
    const blob = new Blob([JSON.stringify(currentCookies, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    await chrome.downloads.download({
      url,
      filename: 'cookies.json',
      saveAs: true,
    });
    showResult('已下載 cookies.json', 'success');
  } catch (error) {
    showResult('下載失敗: ' + error.message, 'error');
  } finally {
    downloadBtn.classList.remove('loading');
    downloadBtn.disabled = false;
  }
}

function showResult(message, type) {
  resultDiv.textContent = message;
  resultDiv.className = `result ${type}`;
}
