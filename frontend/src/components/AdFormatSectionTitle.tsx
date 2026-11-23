import HeadingMarker from '@/components/HeadingMarker'

/** Section heading of an ad format page, with the red marker. */
export default function AdFormatSectionTitle({ children }: { children: string }) {
  return (
    <h2 className="text-[26px] font-bold leading-tight tracking-[0.3px] text-white md:text-[32px]">
      <HeadingMarker />
      {children}
    </h2>
  )
}
