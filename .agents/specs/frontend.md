# Frontend verification

Use for changes to the desktop (`apps/desktop`) or `packages/ui`.

1. Run the real entrypoint: `mise run dev:desktop`, which needs no server.
2. Exercise the affected flow at narrow and wide window sizes, in light and dark color schemes, in
   English and Chinese.
3. Check keyboard navigation, visible focus, and loading, empty and error states.
4. Confirm the webview console has no unexpected error or warning.
5. Capture screenshots of the changed surface into `.agents/evidence/`.

Code inspection and `vp check` alone are not completion evidence. If the real entrypoint cannot run,
report that limit instead of claiming visual verification.
