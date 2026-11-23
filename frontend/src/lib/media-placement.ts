export type MediaSegment = { type: 'text'; text: string } | { type: 'video' } | { type: 'image'; index: number }

// placeMedia lays the article text out with its media: the video after the middle
// paragraph and image i after paragraph round(i * n / (k + 1)). Paragraphs are the
// non-empty lines; at one gap the video comes before the images.
export function placeMedia(text: string, media: { hasVideo: boolean; imageCount: number }): MediaSegment[] {
  const lines = text.split('\n')
  const paragraphLines = lines.flatMap((line, index) => (line.trim() ? [index] : []))
  const paragraphs = paragraphLines.length

  const mediaAtGap = new Map<number, MediaSegment[]>()
  const addAt = (gap: number, segment: MediaSegment) => {
    mediaAtGap.set(gap, [...(mediaAtGap.get(gap) ?? []), segment])
  }
  if (media.hasVideo) addAt(Math.ceil(paragraphs / 2), { type: 'video' })
  for (let index = 0; index < media.imageCount; index++) {
    const gap = Math.round(((index + 1) * paragraphs) / (media.imageCount + 1))
    addAt(Math.min(Math.max(gap, 1), Math.max(paragraphs, 0)), { type: 'image', index })
  }

  if (paragraphs === 0) return mediaAtGap.get(0) ?? []

  const segments: MediaSegment[] = []
  let start = 0
  for (const [gap, gapMedia] of [...mediaAtGap].sort(([a], [b]) => a - b)) {
    const end = paragraphLines[gap - 1] + 1
    pushText(segments, lines.slice(start, end).join('\n'))
    segments.push(...gapMedia)
    start = end
  }
  pushText(segments, lines.slice(start).join('\n'))

  return segments
}

function pushText(segments: MediaSegment[], text: string) {
  const trimmed = text.replace(/^[\r\n]+|[\r\n]+$/g, '')
  if (trimmed.trim()) segments.push({ type: 'text', text: trimmed })
}
