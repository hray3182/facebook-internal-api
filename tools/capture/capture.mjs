#!/usr/bin/env node
/**
 * Semi-manual Facebook doc_id capture.
 *
 * 1. Injects auth.json cookies into headed Chromium
 * 2. Opens the target page(s)
 * 3. Intercepts /api/graphql/ while YOU click around (comments, replies, photos…)
 * 4. Press Enter in this terminal when done → writes docids.json + graphql-log.json
 *
 *   node capture.mjs --auth ../../auth.json --group GID --post PID
 */
import { chromium } from 'playwright';
import fs from 'node:fs';
import path from 'node:path';
import readline from 'node:readline';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

const FRIENDLY_TO_FIELD = {
  ProfileCometTimelineFeedRefetchQuery: 'Posts',
  GroupsCometFeedRegularStoriesPaginationQuery: 'Groups',
  CometSinglePostDialogContentQuery: 'Comments',
  CommentsListComponentsPaginationQuery: 'Comments', // legacy
  Depth1CommentsListPaginationQuery: 'Replies',
  CometPhotoRootContentQuery: 'Photos',
  useCometUFICreateCommentMutation: 'CreateComment',
  useCometUFIDeleteCommentMutation: 'DeleteComment',
};

const INTERESTING_RE =
  /Comment|UFI|Depth|Reply|Feedback|Photo|GroupsCometFeed|TimelineFeed|CreateComment|DeleteComment|SinglePostDialog/i;

function parseArgs(argv) {
  const out = {
    auth: 'auth.json',
    user: process.env.FB_PROBE_USER || '',
    group: process.env.FB_PROBE_GROUP || '',
    post: process.env.FB_PROBE_POST || '',
    startURL: '',
    timeoutMs: 60000,
    out: path.join(__dirname, 'docids.json'),
    dump: path.join(__dirname, 'graphql-log.json'),
  };
  for (let i = 2; i < argv.length; i++) {
    const a = argv[i];
    if (a === '--auth') out.auth = argv[++i];
    else if (a === '--user') out.user = argv[++i];
    else if (a === '--group') out.group = argv[++i];
    else if (a === '--post') out.post = argv[++i];
    else if (a === '--url') out.startURL = argv[++i];
    else if (a === '--out') out.out = argv[++i];
    else if (a === '--dump') out.dump = argv[++i];
    else if (a === '--timeout') out.timeoutMs = Number(argv[++i]);
    else if (a === '--help' || a === '-h') {
      console.log(`Usage: node capture.mjs --auth auth.json [--group ID] [--post ID] [--user ID] [--url URL]

Semi-manual: headed browser + cookie inject + GraphQL intercept.
You operate the UI; press Enter here when finished.`);
      process.exit(0);
    } else {
      console.error(`unknown arg: ${a}`);
      process.exit(2);
    }
  }
  return out;
}

function loadAuth(authPath) {
  const raw = JSON.parse(fs.readFileSync(authPath, 'utf8'));
  if (!Array.isArray(raw.cookies) || raw.cookies.length === 0) {
    throw new Error('auth.json missing cookies[]');
  }
  return raw;
}

function toPlaywrightCookies(cookies) {
  const sameSiteMap = {
    no_restriction: 'None',
    lax: 'Lax',
    strict: 'Strict',
    unspecified: 'Lax',
  };
  return cookies.map((c) => {
    const cookie = {
      name: c.name,
      value: c.value,
      domain: c.domain || '.facebook.com',
      path: c.path || '/',
      secure: c.secure !== false,
      httpOnly: Boolean(c.httpOnly),
      sameSite: sameSiteMap[c.sameSite] || 'Lax',
    };
    if (c.expires && c.expires > 0) cookie.expires = c.expires;
    return cookie;
  });
}

function extractDocID(postData) {
  if (!postData) return null;
  try {
    return new URLSearchParams(postData).get('doc_id');
  } catch {
    const m = String(postData).match(/(?:^|&)doc_id=(\d+)/);
    return m ? m[1] : null;
  }
}

function extractVariables(postData) {
  if (!postData) return null;
  try {
    return new URLSearchParams(postData).get('variables');
  } catch {
    return null;
  }
}

function waitForDone(prompt) {
  const doneFile = path.join(__dirname, '.capture-done');
  try {
    fs.unlinkSync(doneFile);
  } catch {
    // ok
  }

  const rl = readline.createInterface({ input: process.stdin, output: process.stdout });
  let settled = false;

  return new Promise((resolve) => {
    const finish = (reason) => {
      if (settled) return;
      settled = true;
      clearInterval(poll);
      try {
        rl.close();
      } catch {
        // ok
      }
      try {
        fs.unlinkSync(doneFile);
      } catch {
        // ok
      }
      console.log(`\nfinishing (${reason})…`);
      resolve();
    };

    rl.question(prompt, () => finish('Enter'));

    // Also allow: touch tools/capture/.capture-done
    const poll = setInterval(() => {
      if (fs.existsSync(doneFile)) finish('done-file');
    }, 500);

    console.log(`(or: touch ${doneFile})`);
  });
}

function printStatus(found) {
  const mapped = [...found.values()].filter((v) => v.field);
  const known = new Set(Object.values(FRIENDLY_TO_FIELD));
  console.log('\n--- captured so far ---');
  for (const field of known) {
    const hit = mapped.find((v) => v.field === field);
    console.log(hit ? `  OK   ${field}: ${hit.doc_id}` : `  .... ${field}`);
  }
  const extras = [...found.values()].filter((v) => !v.field);
  if (extras.length) {
    console.log('  other interesting:');
    for (const e of extras.slice(-8)) {
      console.log(`       ${e.friendly_name} = ${e.doc_id}`);
    }
  }
  console.log('-----------------------\n');
}

function printGoSnippet(found) {
  const byField = {};
  for (const entry of found.values()) {
    if (entry.field) byField[entry.field] = entry.doc_id;
  }
  const fields = [...new Set(Object.values(FRIENDLY_TO_FIELD))];
  console.log('\n// Suggested DefaultDocIDs values:');
  console.log('var DefaultDocIDs = DocIDs{');
  for (const f of fields) {
    const v = byField[f];
    console.log(v ? `\t${f}: "${v}",` : `\t// ${f}: (not captured)`);
  }
  console.log('}');
}

function startURLFromArgs(args) {
  if (args.startURL) return args.startURL;
  if (args.group && args.post) {
    return `https://www.facebook.com/groups/${args.group}/posts/${args.post}/`;
  }
  if (args.group) return `https://www.facebook.com/groups/${args.group}`;
  if (args.user) return `https://www.facebook.com/profile.php?id=${args.user}`;
  if (args.post) return `https://www.facebook.com/${args.post}`;
  return 'https://www.facebook.com/';
}

async function main() {
  const args = parseArgs(process.argv);
  const authPath = path.resolve(args.auth);
  const auth = loadAuth(authPath);
  const found = new Map();
  const log = [];
  const startURL = startURLFromArgs(args);

  const browser = await chromium.launch({
    headless: false,
    args: ['--disable-blink-features=AutomationControlled'],
  });
  const context = await browser.newContext({
    userAgent:
      'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36',
    viewport: { width: 1400, height: 1000 },
    locale: 'zh-TW',
  });
  await context.addCookies(toPlaywrightCookies(auth.cookies));
  const page = await context.newPage();

  const record = (friendly, docID, variables, note) => {
    if (!friendly || !docID) return;
    const field = FRIENDLY_TO_FIELD[friendly];
    const interesting = Boolean(field) || INTERESTING_RE.test(friendly);
    if (!interesting) return;

    const prev = found.get(friendly);
    if (!prev || prev.doc_id !== docID) {
      console.log(`api: ${field ? `[${field}] ` : ''}${friendly} = ${docID}`);
    }
    found.set(friendly, {
      field: field || null,
      doc_id: docID,
      friendly_name: friendly,
      captured_at: new Date().toISOString(),
      variables_preview: (variables || '').slice(0, 800),
      note,
    });
  };

  page.on('request', (req) => {
    if (!req.url().includes('/api/graphql')) return;
    const friendly = req.headers()['x-fb-friendly-name'] || '';
    const postData = req.postData() || '';
    const docID = extractDocID(postData);
    const variables = extractVariables(postData);
    log.push({
      at: new Date().toISOString(),
      friendly,
      doc_id: docID,
      interesting: INTERESTING_RE.test(friendly) || INTERESTING_RE.test(variables || ''),
      variables: variables,
    });
    record(friendly, docID, variables);
  });

  page.on('response', async (res) => {
    try {
      const req = res.request();
      if (!req.url().includes('/api/graphql')) return;
      const text = await res.text();
      if (!/comment_rendering_instance|CommentsListComponents|replies_connection|currMedia/i.test(text)) {
        return;
      }
      const friendly = req.headers()['x-fb-friendly-name'] || '';
      const docID = extractDocID(req.postData() || '');
      console.log(`api-response comments/photo-ish: ${friendly || '(no name)'} doc_id=${docID}`);
      record(friendly, docID, extractVariables(req.postData() || ''), 'response had comment/photo payload');
    } catch {
      // ignore
    }
  });

  console.log(`injecting cookies from ${authPath}`);
  console.log(`opening ${startURL}`);
  try {
    await page.goto(startURL, { waitUntil: 'domcontentloaded', timeout: args.timeoutMs });
  } catch (err) {
    console.warn(`warn: initial navigation: ${err.message}`);
  }

  console.log(`
============================================================
  SEMI-MANUAL MODE
  - Operate the Facebook window yourself
  - Useful actions: open comments, expand replies, open photo,
    post/delete a test comment if you need mutation doc_ids
  - This terminal will print captured GraphQL as you go
  - When finished, press Enter here to save & quit
============================================================
`);
  printStatus(found);

  // Periodic status while waiting
  const statusTimer = setInterval(() => printStatus(found), 20000);

  await waitForDone('Press Enter to save docids and exit… ');
  clearInterval(statusTimer);

  const payload = {
    captured_at: new Date().toISOString(),
    auth: path.basename(authPath),
    start_url: startURL,
    operations: Object.fromEntries(found),
    by_field: Object.fromEntries(
      [...found.values()].filter((v) => v.field).map((v) => [v.field, v.doc_id])
    ),
  };
  fs.writeFileSync(args.out, JSON.stringify(payload, null, 2));
  fs.writeFileSync(args.dump, JSON.stringify(log, null, 2));

  printStatus(found);
  console.log(`wrote ${args.out}`);
  console.log(`wrote ${args.dump} (${log.length} graphql requests)`);
  printGoSnippet(found);

  await browser.close();
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
