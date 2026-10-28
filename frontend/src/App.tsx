import { Routes, Route } from 'react-router'
import Home from './pages/Home'
import Admin from './pages/Admin'
import Article from './pages/Article'

export default function App() {
  return (
    <Routes>
      <Route path="/" element={<Home />} />
      <Route path="/news/:slug" element={<Article />} />
      <Route path="/admin" element={<Admin />} />
    </Routes>
  )
}
