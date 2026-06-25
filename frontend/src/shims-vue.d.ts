declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<{}, {}, any>
  export default component
}

// nom-ui's useTheme.ts references the `chrome` extension global on a fallback
// path (browser-extension storage). We don't target extensions, but vue-tsc
// type-checks the library's shipped source, so declare a minimal ambient to
// keep the typecheck clean without pulling in @types/chrome.
declare const chrome: any

