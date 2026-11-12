import { Routes, Route } from 'react-router'

import AuthProvider from '@/components/auth/AuthProvider'
import RequireAuth from '@/components/auth/RequireAuth'

import Home from './pages/Home'
import About from './pages/About'
import Admin from './pages/Admin'
import Login from './pages/Login'
import Article from './pages/Article'
import CategoryPage from './pages/Category'
import Feed from './pages/Feed'

export default function App() {
  return (
    <AuthProvider>
      <Routes>
        <Route path="/" element={<Home />} />
        <Route path="/about" element={<About />} />
        <Route path="/sport" element={<CategoryPage category="Спорт" />} />
        <Route path="/international" element={<CategoryPage category="Міжнародні новини" />} />
        <Route path="/politics" element={<CategoryPage category="Політика" />} />
        <Route path="/regional" element={<CategoryPage category="Регіональні новини" />} />
        <Route path="/interview" element={<CategoryPage category="Інтерв'ю" />} />
        <Route path="/news" element={<Feed />} />
        <Route path="/news/:slug" element={<Article />} />
        <Route path="/admin/login" element={<Login />} />
        <Route
          path="/admin"
          element={(
            <RequireAuth>
              <Admin />
            </RequireAuth>
          )}
        />
      </Routes>
    </AuthProvider>
  )
}
