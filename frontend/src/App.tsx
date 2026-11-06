import { Routes, Route } from 'react-router'

import AuthProvider from '@/components/auth/AuthProvider'
import RequireAuth from '@/components/auth/RequireAuth'

import Home from './pages/Home'
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
        <Route path="/sport" element={<CategoryPage category="Спорт" />} />
        <Route path="/international" element={<CategoryPage category="Міжнародні новини" />} />
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
