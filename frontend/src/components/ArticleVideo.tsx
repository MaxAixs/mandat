import type { NewsVideo } from '@/types'

interface ArticleVideoProps {
  video: NewsVideo
  title: string
  /** Cover image shown before a stored file starts playing. */
  poster?: string
}

export default function ArticleVideo({ video, title, poster }: ArticleVideoProps) {
  if (video.kind === 'embed') {
    return (
      <div className="relative aspect-video w-full overflow-hidden border border-mandat-line bg-black">
        <iframe
          src={video.url}
          title={`Відео: ${title}`}
          allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share"
          allowFullScreen
          loading="lazy"
          referrerPolicy="strict-origin-when-cross-origin"
          className="absolute inset-0 h-full w-full"
        />
      </div>
    )
  }

  return (
    <video
      src={video.url}
      poster={poster}
      controls
      preload="metadata"
      playsInline
      className="w-full border border-mandat-line bg-black"
    />
  )
}
