import { Routes, Route } from 'react-router'

import AuthProvider from '@/components/auth/AuthProvider'
import RequireAuth from '@/components/auth/RequireAuth'

import Home from './pages/Home'
import Admin from './pages/Admin'
import Login from './pages/Login'
import Article from './pages/Article'
import CategoryPage from './pages/Category'
import Feed from './pages/Feed'
import Rules from './pages/Rules'
import Advertising from './pages/Advertising'
import BannerAds from './pages/BannerAds'
import Longreads from './pages/Longreads'
import PartnerMaterials from './pages/PartnerMaterials'
import ClientProjects from './pages/ClientProjects'
import Contacts from './pages/Contacts'

export default function App() {
  return (
    <AuthProvider>
      <Routes>
        <Route path="/" element={<Home />} />
        <Route path="/economics" element={<CategoryPage category="Економіка" />} />
        <Route path="/sport" element={<CategoryPage category="Спорт" />} />
        <Route path="/international" element={<CategoryPage category="Міжнародні новини" />} />
        <Route path="/politics" element={<CategoryPage category="Політика" />} />
        <Route path="/regional" element={<CategoryPage category="Регіональні новини" />} />
        <Route path="/interview" element={<CategoryPage category="Інтерв'ю" />} />
        <Route path="/news" element={<Feed />} />
        <Route path="/rules" element={<Rules />} />
        <Route path="/advertising" element={<Advertising />} />
        <Route path="/advertising/banners" element={<BannerAds />} />
        <Route path="/advertising/longreads" element={<Longreads />} />
        <Route path="/advertising/partner-materials" element={<PartnerMaterials />} />
        <Route path="/advertising/client-projects" element={<ClientProjects />} />
        <Route path="/contacts" element={<Contacts />} />
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
