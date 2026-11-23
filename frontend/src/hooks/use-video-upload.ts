import { useCallback, useEffect, useRef, useState } from 'react'

import { getNewsErrorMessage, requestVideoUpload, uploadFile } from '@/api/news'
import { validateVideoFile } from '@/lib/news-validation'

export interface VideoUploadState {
  status: 'idle' | 'uploading' | 'done' | 'error'
  /** Uploaded share of the file, 0..1. */
  progress: number
  fileName: string
  error: string | null
}

export const IDLE_VIDEO_UPLOAD: VideoUploadState = { status: 'idle', progress: 0, fileName: '', error: null }

// useVideoUpload uploads a picked file straight to storage and reports its pending key.
// An unmount aborts a running upload.
export default function useVideoUpload(onUploaded: (key: string) => void) {
  const [state, setState] = useState<VideoUploadState>(IDLE_VIDEO_UPLOAD)
  const controller = useRef<AbortController | null>(null)

  useEffect(() => () => controller.current?.abort(), [])

  const start = useCallback(
    async (file: File) => {
      const invalid = validateVideoFile(file)
      if (invalid) {
        setState({ status: 'error', progress: 0, fileName: file.name, error: invalid })
        return
      }

      controller.current?.abort()
      const current = new AbortController()
      controller.current = current
      setState({ status: 'uploading', progress: 0, fileName: file.name, error: null })

      try {
        const upload = await requestVideoUpload(file)
        await uploadFile(
          upload.upload_url,
          file,
          (progress) => setState((state) => ({ ...state, progress })),
          current.signal,
        )
        if (current.signal.aborted) return

        setState({ status: 'done', progress: 1, fileName: file.name, error: null })
        onUploaded(upload.key)
      } catch (error) {
        if (current.signal.aborted) return

        setState({ status: 'error', progress: 0, fileName: file.name, error: getNewsErrorMessage(error) })
      } finally {
        if (controller.current === current) controller.current = null
      }
    },
    [onUploaded],
  )

  const cancel = useCallback(() => {
    controller.current?.abort()
    controller.current = null
    setState(IDLE_VIDEO_UPLOAD)
  }, [])

  return { state, start, cancel }
}
