# Frontend verification

Use for changes to the console (`apps/web`) or the desktop (`apps/desktop`).

1. Build and run the real entrypoint: `mise run build` then `bin/eyeful-server serve`, or
   `mise run dev:desktop` for the desktop window, which needs no server.
2. Exercise the affected flow signed out and signed in (console only), at phone and desktop widths,
   in light and dark color schemes, in English and Chinese.
3. Check keyboard navigation, visible focus, loading, empty and error states, and that API failures
   show the Problem `detail`.
4. Confirm the browser or webview console has no unexpected error or warning.
5. Capture screenshots of the changed surface into `.agents/evidence/`.

Code inspection and `vp check` alone are not completion evidence. If the real entrypoint cannot run,
report that limit instead of claiming visual verification.
