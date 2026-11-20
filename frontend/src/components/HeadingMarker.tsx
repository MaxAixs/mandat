import { useLayoutEffect, useRef, useState, type CSSProperties } from 'react'

// Red bar before a section heading, sized to the glyph of the heading's first letter
// (letters like «Ф» rise above cap height and drop below the baseline).
export default function HeadingMarker() {
  const ref = useRef<HTMLSpanElement>(null)
  const [glyphBox, setGlyphBox] = useState<CSSProperties>()

  useLayoutEffect(() => {
    const heading = ref.current?.parentElement
    const letter = heading?.textContent?.trim().charAt(0)
    const ctx = document.createElement('canvas').getContext('2d')
    if (!heading || !letter || !ctx) return

    let active = true
    const measure = () => {
      if (!active) return
      const style = getComputedStyle(heading)
      ctx.font = `${style.fontWeight} ${style.fontSize} ${style.fontFamily}`
      const glyph = ctx.measureText(letter)
      setGlyphBox({
        height: glyph.actualBoundingBoxAscent + glyph.actualBoundingBoxDescent,
        verticalAlign: -glyph.actualBoundingBoxDescent,
      })
    }

    measure()
    // The web font may still be loading, and the heading size changes at breakpoints.
    document.fonts.ready.then(measure)
    const observer = new ResizeObserver(measure)
    observer.observe(heading)
    return () => {
      active = false
      observer.disconnect()
    }
  }, [])

  return (
    <span
      ref={ref}
      className="mr-3.5 inline-block h-[1cap] w-2 bg-mandat-red-bright align-baseline"
      style={glyphBox}
      aria-hidden="true"
    />
  )
}
