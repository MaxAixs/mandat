// The category page test compiles App.tsx, which still imports the admin page.
// Admin.tsx passes a `title` prop to a lucide icon; the installed lucide-react
// types do not declare it, and `tsc -b` in the repository fails on the same
// line. This test-only augmentation keeps the category test compiling until
// that unrelated admin typing issue is fixed; it does not change runtime code.
import 'lucide-react'

declare module 'lucide-react' {
  interface LucideProps {
    title?: string
  }
}
