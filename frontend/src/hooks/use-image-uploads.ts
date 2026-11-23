import { useEffect, useRef, useState } from 'react'

import { getNewsErrorMessage, MAX_IMAGES, requestImageUpload, uploadFile } from '@/api/news'
import { validateImageFile, validateImageLink } from '@/lib/news-validation'

export interface ImageItem {
  id: string
  /** What the form sends: a saved image URL, a pending upload key or an image link. */
  value: string
  /** Thumbnail source: the saved URL, the link, or a local preview of the file. */
  preview: string
  status: 'ready' | 'uploading' | 'error'
  /** Uploaded share of the file, 0..1. */
  progress: number
  error: string | null
}

// useImageUploads keeps the editor's ordered image list: saved images, files uploading
// straight to storage and pasted links. Unmounting aborts running uploads.
export default function useImageUploads(initial: string[]) {
  const [items, setItems] = useState<ImageItem[]>(() =>
    initial.map((url, index) => ({
      id: `saved-${index}`,
      value: url,
      preview: url,
      status: 'ready',
      progress: 1,
      error: null,
    })),
  )
  const [notice, setNotice] = useState<string | null>(null)
  const counter = useRef(0)
  const controllers = useRef(new Map<string, AbortController>())
  const previews = useRef(new Map<string, string>())

  useEffect(() => {
    const running = controllers.current
    const objectUrls = previews.current

    return () => {
      running.forEach((controller) => controller.abort())
      objectUrls.forEach((url) => URL.revokeObjectURL(url))
    }
  }, [])

  const patch = (id: string, change: Partial<ImageItem>) => {
    setItems((current) => current.map((item) => (item.id === id ? { ...item, ...change } : item)))
  }

  const upload = async (id: string, file: File) => {
    const controller = new AbortController()
    controllers.current.set(id, controller)

    try {
      const target = await requestImageUpload(file)
      await uploadFile(target.upload_url, file, (progress) => patch(id, { progress }), controller.signal)
      patch(id, { value: target.key, status: 'ready', progress: 1 })
    } catch (error) {
      if (!controller.signal.aborted) patch(id, { status: 'error', error: getNewsErrorMessage(error) })
    } finally {
      controllers.current.delete(id)
    }
  }

  const addFiles = (files: File[]) => {
    const room = MAX_IMAGES - items.length
    setNotice(files.length > room ? `Можна додати не більше ${MAX_IMAGES} фото.` : null)

    const added: ImageItem[] = files.slice(0, Math.max(room, 0)).map((file) => {
      const id = `new-${++counter.current}`
      const invalid = validateImageFile(file)
      const preview = URL.createObjectURL(file)
      previews.current.set(id, preview)
      if (!invalid) void upload(id, file)

      return { id, value: '', preview, status: invalid ? 'error' : 'uploading', progress: 0, error: invalid }
    })
    setItems((current) => [...current, ...added])
  }

  const addLink = (url: string): boolean => {
    const link = url.trim()
    const problem =
      validateImageLink(link) ??
      (items.some((item) => item.value === link) ? 'Це фото вже додано.' : null) ??
      (items.length >= MAX_IMAGES ? `Можна додати не більше ${MAX_IMAGES} фото.` : null)
    setNotice(problem)
    if (problem) return false

    const id = `new-${++counter.current}`
    setItems((current) => [...current, { id, value: link, preview: link, status: 'ready', progress: 1, error: null }])
    return true
  }

  const move = (id: string, direction: 'up' | 'down') => {
    setItems((current) => {
      const from = current.findIndex((item) => item.id === id)
      const to = direction === 'up' ? from - 1 : from + 1
      if (from < 0 || to < 0 || to >= current.length) return current

      const next = [...current]
      ;[next[from], next[to]] = [next[to], next[from]]
      return next
    })
  }

  const remove = (id: string) => {
    controllers.current.get(id)?.abort()
    const preview = previews.current.get(id)
    if (preview) {
      URL.revokeObjectURL(preview)
      previews.current.delete(id)
    }
    setNotice(null)
    setItems((current) => current.filter((item) => item.id !== id))
  }

  return {
    items,
    notice,
    uploading: items.some((item) => item.status === 'uploading'),
    /** The ordered list the form sends; failed uploads are left out. */
    values: items.filter((item) => item.status === 'ready').map((item) => item.value),
    addFiles,
    addLink,
    move,
    remove,
  }
}
