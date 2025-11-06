# Vue 3.5 + Vite 7 Vibespace

## Installed Versions (October 2025)
- **Vue**: 3.5.22
- **Vite**: 7.1.10
- **TypeScript**: 5.9.3
- **pnpm**: 10.18.3
- **Node.js**: 24.x LTS (requires Node 20.19+ or 22.12+)

## Key Features
- **Composition API** with `<script setup>`
- **Vite 7** with ESM-only distribution
- **TypeScript** support out of the box
- **Hot Module Replacement** (HMR)
- **Single File Components** (SFC)
- **Reactivity** with ref() and reactive()

## Commands
- **Dev**: `pnpm dev` (port 5173, HMR enabled)
- **Build**: `pnpm build`
- **Preview**: `pnpm preview`
- **Type check**: `vue-tsc --noEmit`

## Best Practices
- ✅ Use `<script setup>` syntax (concise, performant)
- ✅ TypeScript: Use defineProps with types
- ✅ Reactivity: `ref()` for primitives, `reactive()` for objects
- ✅ Computed properties: `computed(() => ...)`
- ✅ Lifecycle: `onMounted()`, `onUnmounted()`, etc.
- ✅ Styling: Scoped styles `<style scoped>`

## Project Structure
```
src/
├── App.vue           # Root component
├── main.ts          # Entry point
├── components/      # Vue components
├── views/           # Page views
├── router/          # Vue Router (if added)
└── assets/          # Static assets
```

## Add Libraries
- **Vue Router**: `pnpm add vue-router@latest`
- **Pinia** (state): `pnpm add pinia@latest`
- **VueUse**: `pnpm add @vueuse/core`

## Resources
- [Vue 3 Docs](https://vuejs.org/guide/)
- [Vite 7 Docs](https://vite.dev/guide/)
- [TypeScript with Vue](https://vuejs.org/guide/typescript/)
