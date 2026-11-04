import { createFileRoute } from '@tanstack/solid-router'
import { NotFound } from '../components/NotFound'
import { site } from '../site'

export const Route = createFileRoute('/404')({
  head: () => ({ meta: [{ title: `找不到页面 · ${site.name}` }] }),
  component: NotFound,
})
