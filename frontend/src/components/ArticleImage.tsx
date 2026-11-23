import { useEffect, useState } from 'react'
import { X } from 'lucide-react'

interface ArticleImageProps {
  src: string
  alt: string
}

// ArticleImage shows an article image inline; a click opens it full screen.
export default function ArticleImage({ src, alt }: ArticleImageProps) {
  const [open, setOpen] = useState(false)

  useEffect(() => {
    if (!open) return

    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setOpen(false)
    }
    // The page behind the viewer must not scroll while it is open.
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    window.addEventListener('keydown', closeOnEscape)

    return () => {
      window.removeEventListener('keydown', closeOnEscape)
      document.body.style.overflow = previousOverflow
    }
  }, [open])

  return (
    <>
      <button
        type="button"
        aria-label="Відкрити фото"
        onClick={() => setOpen(true)}
        className="block w-full cursor-zoom-in border border-mandat-line bg-mandat-surface-2"
      >
        <img src={src} alt={alt} loading="lazy" className="max-h-[560px] w-full object-cover" />
      </button>

      {open && (
        <div
          role="dialog"
          aria-modal="true"
          aria-label={alt}
          onClick={(event) => event.target === event.currentTarget && setOpen(false)}
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/90 p-4 md:p-10"
        >
          <img src={src} alt={alt} className="max-h-full max-w-full object-contain" />
          <button
            type="button"
            aria-label="Закрити"
            autoFocus
            onClick={() => setOpen(false)}
            className="absolute right-4 top-4 p-2 text-white/80 transition-colors hover:text-white"
          >
            <X className="size-7" />
          </button>
        </div>
      )}
    </>
  )
}
