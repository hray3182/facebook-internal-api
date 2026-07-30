# DocID tooling

## 1. Export cookies (`extension/`)

Chrome extension adapted from joaimy.com `fb-login-extension`.

1. Chrome → `chrome://extensions` → Developer mode → Load unpacked → select `extension/`
2. Open and log into `facebook.com`
3. Click the extension → **下載 auth.json**

`auth.json` contains `cookies`, `fb_dtsg`, and `lsd`.

## 2. Probe current doc_ids (`cmd/probe`)

```bash
go run ./cmd/probe -auth auth.json \
  -user USER_ID \
  -group GROUP_ID \
  -post POST_ID
```

Known live fixture (when auth is valid):

```bash
# https://www.facebook.com/groups/1635204946735429/posts/4400009946921568
go run ./cmd/probe -auth auth.json \
  -group 1635204946735429 \
  -post 4400009946921568

go test -run TestLive_ListComments -v
```

Exit code `1` means at least one live probe failed (doc_id may have rotated).

## 3. Capture fresh doc_ids (`tools/capture`) — semi-manual

Injects `auth.json` cookies into a **headed** Chromium, opens the target page,
and intercepts `/api/graphql/` while **you** click around. Press Enter in the
terminal when done.

```bash
cd tools/capture
npm install
npx playwright install chromium

node capture.mjs --auth ../../auth.json \
  --group 1635204946735429 \
  --post 1722750337980889
# or: --url 'https://www.facebook.com/groups/...'
```

While you browse, the terminal prints matches like `[Comments] … = <doc_id>`.

Writes:

- `docids.json` — mapped ops + suggested `DefaultDocIDs` snippet
- `graphql-log.json` — full GraphQL log (for unknown friendly names)

Suggested manual actions:

1. Open a post → click comments / sort / view more（應出現 `CommentsListComponentsPaginationQuery`）
2. Expand a reply thread
3. Open a photo in the viewer
4. (optional) open permalink dialog for `CometSinglePostDialogContentQuery` → `CommentsDialog`
5. (optional) create then delete a throwaway comment for mutation doc_ids
