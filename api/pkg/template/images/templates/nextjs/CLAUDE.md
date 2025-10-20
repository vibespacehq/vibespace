# Next.js 15.5 Workspace

## Installed Versions (October 2025)
- **Next.js**: 15.5.5
- **React**: 19.x
- **TypeScript**: 5.9.3
- **Tailwind CSS**: 4.1.14
- **Turbopack**: Stable (default bundler)
- **pnpm**: 10.18.3
- **Node.js**: 24.x LTS

## Key Features
- **App Router** (default in Next.js 15)
- **React 19** with Server Components
- **Turbopack** (5-10x faster builds)
- **Server Actions** for mutations
- **Streaming & Suspense** for progressive rendering
- **Image Optimization** (next/image)
- **Font Optimization** (next/font)

## Commands
- **Dev**: `pnpm dev` (port 3000, Turbopack enabled)
- **Build**: `pnpm build`
- **Start**: `pnpm start` (production mode)
- **Lint**: `pnpm lint`
- **Type check**: `pnpm type-check`

## Best Practices
- ✅ Use Server Components by default (faster, smaller bundle)
- ✅ Add 'use client' only when needed (interactivity, hooks)
- ✅ API Routes: `app/api/[route]/route.ts`
- ✅ Metadata: Export metadata object from layouts/pages
- ✅ Data Fetching: async/await in Server Components
- ✅ Styling: Tailwind utility classes

## Project Structure
```
app/
├── layout.tsx         # Root layout
├── page.tsx          # Home page
├── api/              # API routes
├── (routes)/         # Route groups
components/           # React components
lib/                  # Utilities
public/              # Static assets
```

## Resources
- [Next.js 15 Docs](https://nextjs.org/docs)
- [React 19 Docs](https://react.dev)
- [Tailwind CSS 4.1](https://tailwindcss.com)
- [Turbopack](https://nextjs.org/docs/architecture/turbopack)
